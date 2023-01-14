package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/btree"
)

type void = struct{}

type Account struct {
	Account struct {
		UserName string `json:"username"`
	} `json:"account"`
}

type User struct {
	ScreenName string `json:"screen_name"`
}

type Tweet struct {
	Text            string `json:"text"`
	FullText        string `json:"full_text"`
	IdStr           string `json:"id_str"`
	CreatedAt       string `json:"created_at"`
	RetweetedStatus *Tweet `json:"retweeted_status"`
	User            *User  `json:"user"`
	TweetNewVersion *Tweet `json:"tweet,omitempty"`
}

var (
	flagDir        = flag.String("d", ".", "root directory to output")
	flagBlobMaster = flag.String("b", ".", "relative path from index to each markdown")
)

var replaceTable = strings.NewReplacer(
	"\n", "  \n",
	"*", "\\*",
	"-", "\\-",
	"[", "\\[",
	"]", "\\]",
)

var ymd btree.Map[string, *btree.Map[string, *btree.Set[string]]]
var createdFile = map[string]void{}
var doneTweet = map[uint64]void{}

func readTweetJSON(r io.Reader, root, dateFormat, user string) error {
	bin, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	var tweets []Tweet
	err = json.Unmarshal(bin, &tweets)
	if err != nil {
		return err
	}

	var stock btree.Map[string, []Tweet]

	for _, tw := range tweets {
		if tw.TweetNewVersion != nil {
			tw = *tw.TweetNewVersion
		}
		id64, err := strconv.ParseUint(tw.IdStr, 10, 64)
		if err != nil {
			return fmt.Errorf("id_str:\"%s\": %w\n", tw.IdStr, err)
		}
		if _, ok := doneTweet[id64]; ok {
			continue
		}
		doneTweet[id64] = void{}
		stamp, err := time.Parse(dateFormat, tw.CreatedAt)
		if err != nil {
			return err
		}
		stamp = stamp.Local()
		key := fmt.Sprintf("%04d%02d%02d", stamp.Year(), stamp.Month(), stamp.Day())
		stock1, _ := stock.Get(key)
		stock1 = append(stock1, tw)
		stock.Set(key, stock1)
	}
	iter := stock.Iter()
	for ok := iter.First(); ok; ok = iter.Next() {
		dt := iter.Key()
		tweets := iter.Value()

		year := dt[:4]
		month := dt[4:6]
		mday := dt[6:]

		ymPath := filepath.Join(root, year, month)
		os.MkdirAll(ymPath, 666)
		articlePath := filepath.Join(ymPath, mday) + ".md"
		//fmt.Fprintln(os.Stderr, filepath.ToSlash(articlePath))

		var fd *os.File
		if _, ok := createdFile[articlePath]; ok {
			fd, err = os.OpenFile(articlePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "%s: reopened(for %d tweets)\n",
				filepath.ToSlash(articlePath), len(tweets))
		} else {
			fd, err = os.Create(articlePath)
			if err != nil {
				return err
			}
			fmt.Fprintf(fd, "### %s/%s/%s (%d tweets)\n\n",
				year, month, mday, len(tweets))
			createdFile[articlePath] = void{}
		}
		monthMap, ok := ymd.Get(year)
		if !ok {
			monthMap = new(btree.Map[string, *btree.Set[string]])
			ymd.Set(year, monthMap)
		}
		dayMap, ok := monthMap.Get(month)
		if !ok {
			dayMap = new(btree.Set[string])
			monthMap.Set(month, dayMap)
		}
		dayMap.Insert(mday)

		for _, tw := range tweets {
			text := tw.Text
			if text == "" {
				text = tw.FullText
			}
			text = replaceTable.Replace(text)
			stamp, err := time.Parse(dateFormat, tw.CreatedAt)
			if err != nil {
				fd.Close()
				return err
			}
			var url string
			if tw.User != nil {
				url = fmt.Sprintf("https://twitter.com/%s/status/%s",
					tw.User.ScreenName, tw.IdStr)
			} else {
				url = fmt.Sprintf("https://twitter.com/%s/status/%s",
					user, tw.IdStr)
			}
			stamp = stamp.Local()
			fmt.Fprintf(fd, "%s  \n*[%v](%s)*\n\n", text, stamp, url)
		}
		fd.Close()
	}
	return nil
}

func readTweetJs(f *zip.File, skipChar byte, root, dateFormat, user string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	br := bufio.NewReader(rc)
	_, err = br.ReadString(skipChar)
	if err != nil {
		return err
	}
	return readTweetJSON(br, root, dateFormat, user)
}

func readAccountJs(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	br := bufio.NewReader(rc)
	_, err = br.ReadString('=')
	if err != nil {
		return "", err
	}
	bin, err := io.ReadAll(br)
	if err != nil {
		return "", err
	}
	account := []*Account{}
	err = json.Unmarshal(bin, &account)
	if err != nil {
		return "", err
	}
	if len(account) >= 1 {
		return account[0].Account.UserName, nil
	}
	return "", errors.New("UserName not found")
}

func readZip(zipFname, root string) error {
	z, err := zip.OpenReader(zipFname)
	if err != nil {
		return err
	}
	defer z.Close()
	var username string
	for _, f := range z.File {
		if path.Ext(f.Name) != ".js" {
			continue
		}
		if f.Name == "account.js" {
			username, err = readAccountJs(f)
		} else if path.Dir(f.Name) == "data/js/tweets" {
			err = readTweetJs(f, '\n', root, "2006-01-02 15:04:05 -0700", username)
		} else if ok, err := filepath.Match("data/tweets-part*.js", f.Name); (err == nil && ok) || f.Name == "tweet.js" || f.Name == "data/tweets.js" {
			err = readTweetJs(f, '=', root, "Mon Jan 02 15:04:05 -0700 2006", username)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	return nil
}

func mains(args []string) error {
	for _, arg1 := range args {
		fnames, err := filepath.Glob(arg1)
		if err != nil {
			fnames = []string{arg1}
		}
		for _, fn := range fnames {
			if err := readZip(fn, *flagDir); err != nil {
				return err
			}
		}
	}
	ymd.Reverse(func(y string, ms *btree.Map[string, *btree.Set[string]]) bool {
		fmt.Printf("### %s\n\n", y)
		ms.Reverse(func(m string, ds *btree.Set[string]) bool {
			fmt.Printf("* %s |", m)
			ds.Scan(func(d string) bool {
				fmt.Printf(" [%s](%s.md)", d, path.Join(*flagBlobMaster, y, m, d))
				return true
			})
			fmt.Println()
			return true
		})
		return true
	})
	return nil
}

func main() {
	flag.Parse()
	if err := mains(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

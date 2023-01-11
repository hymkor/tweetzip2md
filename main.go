package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hymkor/go-sortedkeys"
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
}

var replaceTable = strings.NewReplacer(
	"\n", "  \n",
	"*", "\\*",
	"-", "\\-",
	"[", "\\[",
	"]", "\\]",
)

var ymd = map[string]map[string]map[string]void{}
var created = map[string]void{}

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

	stock := map[string][]Tweet{}

	for _, tw := range tweets {
		if tw.RetweetedStatus != nil {
			//continue
		}
		stamp, err := time.Parse(dateFormat, tw.CreatedAt)
		if err != nil {
			return err
		}
		stamp = stamp.Local()
		key := fmt.Sprintf("%04d%02d%02d", stamp.Year(), stamp.Month(), stamp.Day())
		stock[key] = append(stock[key], tw)
	}
	for p := sortedkeys.New(stock); p.Range(); {
		dt := p.Key
		tweets := p.Value

		year := dt[:4]
		month := dt[4:6]
		mday := dt[6:]

		ymPath := filepath.Join(root, year, month)
		os.MkdirAll(ymPath, 666)
		articlePath := filepath.Join(ymPath, mday) + ".md"
		fmt.Fprintln(os.Stderr, filepath.ToSlash(articlePath))

		var fd *os.File
		if _, ok := created[articlePath]; ok {
			fd, err = os.OpenFile(articlePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
		} else {
			fd, err = os.Create(articlePath)
			if err != nil {
				return err
			}
			fmt.Fprintf(fd, "### %s/%s/%s (%d tweets)\n\n",
				year, month, mday, len(tweets))
		}
		monthMap, ok := ymd[year]
		if !ok {
			monthMap = map[string]map[string]void{}
			ymd[year] = monthMap
		}
		dayMap, ok := monthMap[month]
		if !ok {
			dayMap = map[string]void{}
			monthMap[month] = dayMap
		}
		dayMap[mday] = void{}

		for i := len(tweets) - 1; i >= 0; i-- {
			tw := tweets[i]
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
			fmt.Fprintf(fd, "%s  \n[%v](%s)\n\n", text, stamp, url)
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
	return readTweetJSON(br, ".", dateFormat, user)
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

func readZip(zipFname string) error {
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
			err = readTweetJs(f, '\n', ".", "2006-01-02 15:04:05 -0700", username)
		} else if f.Name == "tweet.js" {
			err = readTweetJs(f, '=', ".", "Mon Jan 02 15:04:05 -0700 2006", username)
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
			if err := readZip(fn); err != nil {
				return err
			}
		}
	}
	for y := sortedkeys.New(ymd); y.Range(); {
		fmt.Printf("### %s\n\n", y.Key)
		for m := sortedkeys.New(y.Value); m.Range(); {
			fmt.Printf("* %s", m.Key)
			dem := '|'
			for d := sortedkeys.New(m.Value); d.Range(); {
				fmt.Printf("%c[%s](%s.md)", dem, d.Key, path.Join(y.Key, m.Key, d.Key))
				dem = ' '
			}
			fmt.Println()
		}
	}
	return nil
}

func main() {
	if err := mains(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

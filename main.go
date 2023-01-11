package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hymkor/go-sortedkeys"
)

type Tweet struct {
	Text            string `json:"text"`
	IdStr           string `json:"id_str"`
	CreatedAt       string `json:"created_at"`
	RetweetedStatus *Tweet `json:"retweeted_status"`
}

var dateFormat = "2006-01-02 15:04:05 -0700"

var replaceTable = strings.NewReplacer(
	"\n", "  \n",
	"*", "\\*",
	"-", "\\-",
	"[", "\\[",
	"]", "\\]",
)

var created = map[string]struct{}{}

func tryOneMonth(r io.Reader, root string) error {
	br := bufio.NewReader(r)
	_, err := br.ReadString('\n')
	if err != nil {
		return fmt.Errorf("Skip Header: %w", err)
	}
	bin, err := io.ReadAll(br)
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
		for i := len(tweets) - 1; i >= 0; i-- {
			tw := tweets[i]
			text := replaceTable.Replace(tw.Text)
			stamp, err := time.Parse(dateFormat, tw.CreatedAt)
			if err != nil {
				fd.Close()
				return err
			}
			stamp = stamp.Local()
			fmt.Fprintf(fd, "%s  \n(%v)\n\n", text, stamp)
		}
		fd.Close()
	}
	return nil
}

func main() {
	if err := tryOneMonth(os.Stdin, "."); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

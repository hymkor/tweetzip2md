tweetzip2md
===========

Convert from Twitter's downloaded archive to markdown files

```
./tweetzip2md *.zip > README.md
```

tweetzip2md creates markdown files whose name are like `2019/12/30.md` and outputs index to STDOUT.

```
$ tweetzip2md.exe -h
Usage of tweetzip2md.exe:
  -d string
        root directory to output (default ".")
  -show-reopen-date
        show date on opening for append
  -show-source-name
        show source JSON in zip-file
```

Supported JSON Version
----------------------

The versions used on downloaded zip archive at these dates.

- Jan.13, 2023
- Jul.26, 2018
- Jul.12, 2018
- Jun.13, 2017
- Sep.19, 2017
- Dec.12, 2017

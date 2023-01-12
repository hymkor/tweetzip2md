tweetzip2md
===========

Convert from Twitter's downloaded archive to markdown files

```
./tweetzip2md *.zip > README.md
```

tweetzip2md creates markdown files whose name are like `./2019/12/30.md` and outputs index to STDOUT.

```
$ tweetzip2md -h
Usage of tweetzip2md:
  -b string
        relative path from index to each markdown (default ".")
  -d string
        root directory to output (default ".")
```

If you want to make markdowns to put on GitHub, for example:
```
tweetzip2md -d ../tweets -b blob/master *.zip > ../tweets/README.md
```

# s3grep

`s3grep` is an overly simple command to scan files in S3 for lines matching a
 search query. It hacks S3 Select to find lines in a file that contain a search string. 

> NOTE: This is just a proof of concept / toy project for now.

## Motivations

This was written in order to simplify finding lines in a file that match a string. Without this command
I've resorted to downloading the whole file and piping to grep. This uses a lot of bandwidth, and can be
slow. And alternative is to use S3 Select to find lines containing a string, and that's exactly what this
command does. 

Note that there is a cost associated with scanning a file in S3 per byte scanned. If the files are compressed,
the cost can be lowered as there will be less bytes scanned. 

## Limitations

- We only scan a single file at a time (this could change - ideally it should detect if the path is a single file or prefix, and in the case of a prefix just scan all matching files).
- Only simple string matches are supported.
- Files with null bytes aren't supported. 
- The credentials setup in the default environment are the only ones used. For fancy profiles use something like aws-runas.

## Installation

- Install golang: <https://golang.org>
- Setup a go environment: <https://golang.org/doc/code.html>
- Use go get to install
```bash
$ go get github.com/markwatson/s3grep
```

## Usage
See `s3grep -help`:
```
  -match string
        The text to match on.
  -path string
        The S3 path to scan.
```



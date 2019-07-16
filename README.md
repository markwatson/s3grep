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

## Limitations

- We only scan a single file at a time.
- Only simple string matches are supported.
- Files with null bytes aren't supported. 
- The credentials setup in the default environment are the only ones used. For fancy profiles use something like aws-runas.

## Planned Work

- Scan paths to search multiple files. Ideally it should detect if the path is a single file or prefix, and in the case of a prefix just scan all matching files.
- Handle true regex patterns to find in files. We will have to use some combination of pruning files with S3 queries & then filtering from there to limit data transfers.
- Consider indexing S3 lists / files so we don't have to re-pull data down each time. 
- Support files with null bytes. This is currently difficult because of how we hack S3 select to work for our purpose. 
- Possibly detect schemas and support other types of ad hoc queries over files. This would kinda be moving away from a "grep" model. 
- Improve the credential / region handling to be more flexible. Currently we just use the default in the user's env, but it would be nice if we supported profiles, etc. 
- Add the ability to estimate cost upfront, and let the user set limits. By storing a mapping of costs in various regions, we could limit the number of `list` queries, and avoid scanning super large files without the user explicitly deciding to do so. Additionally, we could consider the trade-off in cost and time between pulling a file and scanning v. letting AWS scan it for us. 
package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"net/url"
	"os"
	"strings"
)

// Print an error message then quit.
func exitErrorf(msg string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

// Get the S3 client.
func getS3() *s3.S3 {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := s3.New(sess)

	return svc
}

// Utility to trim off first slash if it exists
func trimInitialSlash(s string) string {
	if strings.HasPrefix(s, "/") {
		return s[1:]
	} else {
		return s
	}
}

// Parse an S3 path into the bucket and prefix.
func parseS3Path(path string) (error, string, string) {
	u, err := url.Parse(path)

	if err != nil {
		return err, "", ""
	} else if u.Scheme != "s3" {
		return fmt.Errorf("scheme '%s' is not s3", u.Scheme), "", ""
	} else {
		return nil, u.Host, trimInitialSlash(u.Path)
	}
}

// Return the compression type for S3 select based on the object suffix.
func getCompression(key string) string {
	if strings.HasSuffix(key, ".gz") {
		return "GZIP"
	} else if strings.HasSuffix(key, ".bz2") {
		return "BZIP2"
	} else {
		return "NONE"
	}
}

// Get the parameters to scan an object using S3 select for a static string.
func scanObjectParams(bucket string, key string, exp string) *s3.SelectObjectContentInput {
	compression := getCompression(key)

	return &s3.SelectObjectContentInput{
		Bucket:         aws.String(bucket),
		Key:            aws.String(key),
		ExpressionType: aws.String(s3.ExpressionTypeSql),
		Expression:     aws.String("select * from s3object s where s._1 like '%" + exp + "%'"),
		InputSerialization: &s3.InputSerialization{
			CSV: &s3.CSVInput{
				FieldDelimiter:  aws.String("\000"),
				RecordDelimiter: aws.String("\n"),
				FileHeaderInfo:  aws.String(s3.FileHeaderInfoNone),
			},
			CompressionType: aws.String(compression),
		},
		OutputSerialization: &s3.OutputSerialization{
			CSV: &s3.CSVOutput{
				QuoteCharacter:       aws.String(""),
				QuoteEscapeCharacter: aws.String(""),
				FieldDelimiter:       aws.String(""),
			},
		},
	}
}

// Check for special case where we have a "folder"
// TODO: Figure out a less hacky way to detect this - S3 isn't supposed to have a concept of folders...
func isFolderKey(key string) bool {
	return strings.HasSuffix(key, "/")
}

// Iterate over items at an S3 prefix. If the process function returns `false`, then break the loop.
// This function currently won't return "folder" keys ending in `/`.
func iterObjects(svc *s3.S3, bucket string, prefix string, process func(key string) bool) error {
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	keepGoing := true
	err := svc.ListObjectsV2Pages(params,
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			objects := page.Contents
			for _, object := range objects {
				if !isFolderKey(*object.Key) {
					keepGoing = process(*object.Key)
					if !keepGoing {
						break
					}
				}
			}
			return keepGoing
		})

	return err
}

// Given an S3 bucket / key and expression, scan the file and print out all matching lines.
func matchS3(svc *s3.S3, bucket string, key string, exp string, printKey bool) error {
	// Special case where we try to scan a "folder"
	if isFolderKey(key) {
		return awserr.New(s3.ErrCodeNoSuchKey, "Considering ending in '/' to not be a real key", nil)
	}

	params := scanObjectParams(bucket, key, exp)

	// Issue S3 Select
	resp, err := svc.SelectObjectContent(params)
	if err != nil {
		return err
	}

	// TODO print key before each match line
	if printKey {
		fmt.Println("=== " + key + " ===")
	}

	// Loop over results
	for event := range resp.EventStream.Events() {
		switch v := event.(type) {
		case *s3.RecordsEvent:
			// s3.RecordsEvent.Records is a byte slice of select records
			fmt.Println(string(v.Payload))
		}
	}

	// Close and report all errors
	if err := resp.EventStream.Close(); err != nil {
		return fmt.Errorf("failed to read from SelectObjectContent EventStream, %v", err)
	}

	return nil
}

// Iterate over all objects under a prefix, and scan them
func scanAndMatchS3(svc *s3.S3, bucket string, key string, exp string) error {
	var internalError error = nil

	err := iterObjects(svc, bucket, key, func(objKey string) bool {
		internalError = matchS3(svc, bucket, objKey, exp, true)
		if internalError != nil {
			return false
		}

		return true
	})

	if internalError != nil {
		return internalError
	} else {
		return err
	}
}

func main() {
	svc := getS3()

	path := flag.String("path", "", "The S3 path to scan.")
	match := flag.String("match", "", "The text to match on.")

	flag.Parse()
	if *path == "" {
		exitErrorf("You must specify a path. See -help")
	}
	if *match == "" {
		exitErrorf("You must specify text to match on. See -help")
	}

	err, bucket, key := parseS3Path(*path)
	if err != nil {
		exitErrorf("Unable to parse path: %v", err)
	}

	// Convoluted way of falling back to prefix scanning ahead
	err = matchS3(svc, bucket, key, *match, false)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				// Let's try again, but this time scanning a prefix first
				err = scanAndMatchS3(svc, bucket, key, *match)
				if err != nil {
					exitErrorf("AWS Client Error listing prefixes and scanning: %v", err)
				}
			default:
				exitErrorf("AWS Client Error scanning: %v", err)
			}
		} else {
			exitErrorf("Generic Error scanning: %v", err)
		}
	}
}

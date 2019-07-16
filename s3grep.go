package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"net/url"
	"os"
	"strings"
)

func exitErrorf(msg string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func getS3() *s3.S3 {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := s3.New(sess)

	return svc
}

func parseS3Path(path string) (error, string, string) {
	u, err := url.Parse(path)

	if err != nil {
		return err, "", ""
	} else if u.Scheme != "s3" {
		return fmt.Errorf("scheme '%s' is not s3", u.Scheme), "", ""
	} else {
		return nil, u.Host, u.Path
	}
}

func getCompression(key string) string {
	if strings.HasSuffix(key, ".gz") {
		return "GZIP"
	} else if strings.HasSuffix(key, ".bz2") {
		return "BZIP2"
	} else {
		return "NONE"
	}
}

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

func matchS3(svc *s3.S3, bucket string, key string, exp string) error {
	params := scanObjectParams(bucket, key, exp)

	// Issue S3 Select
	resp, err := svc.SelectObjectContent(params)
	if err != nil {
		return err
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

	err = matchS3(svc, bucket, key, *match)
	if err != nil {
		exitErrorf("Error scanning: %v", err)
	}
}

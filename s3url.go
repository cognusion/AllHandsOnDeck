// +build go1.4

package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// The parts of an S3 URL
type S3Url struct {
	Bucket string
	Path   string
	File   string
	Params string
}

// Convert the Params string to a []string
func (s *S3Url) ParamList() []string {
	return strings.Split(s.Params, "&")
}

// Convert an S3 URL into a S3Url structure
func s3UrlToParts(url string) (sr *S3Url) {

	if strings.HasPrefix(url, "http") {
		// Long nasty URL, but we still need the parts
		tparts := strings.SplitN(url, "?", 2)
		parts := strings.Split(tparts[0], "/")
		bucket := strings.TrimSuffix(parts[2], ".s3.amazonaws.com")
		file := parts[len(parts)-1]
		path := "/" + strings.Join(parts[3:len(parts)], "/")
		sr = &S3Url{
			Bucket: bucket,
			Path:   path,
			File:   file,
		}
		if len(tparts) > 1 {
			sr.Params = tparts[1]
		}
	} else {
		// s3:// URL or who knows. Try.
		url = strings.TrimPrefix(url, "s3://")
		parts := strings.SplitN(url, "/", 2)
		moar := strings.Split(url, "/")
		sr = &S3Url{
			Bucket: parts[0],
			Path:   "/" + parts[1],
			File:   moar[len(moar)-1],
		}
	}
	return sr
}

func generateS3Url(bucket string, filePath string, awsAccessKeyID string,
	awsSecretAccessKey string, httpMethod string, minuteExpire int) string {

	if httpMethod == "" {
		httpMethod = "GET"
	}

	// If you need to support multiple regions, you'll need to pass it in,
	// and mangle this var correspondingly
	endPoint := "s3.amazonaws.com"

	// In case it's not there, make sure it is
	if strings.HasPrefix(filePath, "/") == false {
		filePath = "/" + filePath
	}

	// String of the Epoch offset to expire the link in
	expire := fmt.Sprintf("%d", int64(time.Now().Unix())+int64(minuteExpire*60))

	// Raw string to use as the signature
	sigString := httpMethod + "\n\n\n" + expire + "\n" + "/" + bucket + filePath

	// We take the base64-encoded HMAC-SHA1 sum of the sigString,
	// using the SecretKey as the key
	mac := hmac.New(sha1.New, []byte(awsSecretAccessKey))
	mac.Write([]byte(sigString))
	msum := mac.Sum(nil)
	signature := base64.StdEncoding.EncodeToString(msum)

	// Compose the query
	query := "AWSAccessKeyId=" + url.QueryEscape(awsAccessKeyID) + "&Expires=" + expire + "&Signature=" + url.QueryEscape(signature)

	// Return the URL
	return "https://" + bucket + "." + endPoint + filePath + "?" + query
}

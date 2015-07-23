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

func s3UrlToParts(url string) (string, string) {
	url = strings.TrimPrefix(url,"s3://")
	parts := strings.SplitN(url,"/",2)
	return parts[0],"/"+parts[1]
}

func generateS3Url(bucket string, filePath string, awsAccessKeyID string,
	awsSecretAccessKey string, httpMethod string, minuteExpire int) string {

	if httpMethod == "" {
		httpMethod = "GET"
	}

	// If you need to support multiple regions, you'll need to pass it in,
	// and mangle this var correspondingly
	endPoint := "s3.amazonaws.com"

	// In case it's there, make sure it's not
	filePath = strings.TrimPrefix(filePath,"/")
	
	// String of the Epoch offset to expire the link in
	expire := fmt.Sprintf("%d", int64(time.Now().Unix())+int64(minuteExpire*60))

	// Raw string to use as the signature
	sigString := httpMethod + "\n\n\n" + expire + "\n" + "/" + bucket + "/" + filePath

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

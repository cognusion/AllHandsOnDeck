package main

import (
	"testing"
)

var string1 string = "s3://thisisthebucket/path/to/file.zip"
var string2 string = "http://thisisthebucket.s3.amazonaws.com/path/to/file.zip?someparam=somevalue"
var string3 string = "https://thisisthebucket.s3.amazonaws.com/path/to/file.zip?someparam=somevalue"

func TestS3_TokenGen(t *testing.T) {
	bucket := "thisisthbucket"
	path := "/this/is/the/path/to/file.zip"
	accessKey := "1234567890"
	secretKey := "0987654321"
	op := "GET"
	lifeTime := 60

	url := generateS3Url(bucket, path,
		accessKey, secretKey, op, lifeTime)

	s3u := s3UrlToParts(url)
	if s3u.Bucket != bucket {
		t.Errorf("Expected bucket '%s', got %s\n", bucket, s3u.Bucket)
	}
	if s3u.Path != path {
		t.Errorf("Expected '%s', got %s\n", path, s3u.Path)
	}
	
	// AWSAccessKeyId=1234567890
	// Expires=1438229728
	// Signature=ed5UfNSZI30BApl2IWEHhjs0ujQ%3D
	params := s3u.ParamList()
	if len(params) != 3 {
		t.Error("Expected 3 params in list, got ", params)
	}
	// TODO: better
}

func TestS3_S3Parts(t *testing.T) {
	s3u := s3UrlToParts(string1)
	if s3u.Bucket != "thisisthebucket" {
		t.Error("Expected bucket 'thisisthebucket', got ", s3u.Bucket)
	}
	if s3u.Path != "/path/to/file.zip" {
		t.Error("Expected '/path/to/file.zip', got ", s3u.Path)
	}
	if s3u.File != "file.zip" {
		t.Error("Expected 'file.zip', got ", s3u.File)
	}
}

func TestS3_HTTPParts(t *testing.T) {
	http := s3UrlToParts(string2)
	https := s3UrlToParts(string3)

	// Test equality between HTTP and HTTPS
	if http.Bucket != https.Bucket {
		t.Errorf("Bucket diff, HTTP: %s HTTPS: %s\n", http.Bucket, https.Bucket)
	}
	if http.Path != https.Path {
		t.Errorf("Path diff, HTTP: %s HTTPS: %s\n", http.Path, https.Path)
	}
	if http.File != https.File {
		t.Errorf("File diff, HTTP: %s HTTPS: %s\n", http.File, https.File)
	}
	if http.Params != https.Params {
		t.Errorf("Params diff, HTTP: %s HTTPS: %s\n", http.Params, https.Params)
	}

	// Test all the parts
	if http.Bucket != "thisisthebucket" {
		t.Error("Expected bucket 'thisisthebucket', got ", http.Bucket)
	}
	if http.Path != "/path/to/file.zip" {
		t.Error("Expected '/path/to/file.zip', got ", http.Path)
	}
	if http.File != "file.zip" {
		t.Error("Expected 'file.zip', got ", http.File)
	}
	if http.Params != "someparam=somevalue" {
		t.Error("Expected 'someparam=somevalue', got ", http.Params)
	}
}

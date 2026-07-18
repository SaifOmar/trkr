#!/bin/bash
set -e
GOOS=windows GOARCH=amd64 go build -o trkr.exe .
mv trkr.exe ~/Windows/

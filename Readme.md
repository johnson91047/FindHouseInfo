# Find House Info

## Description

This is a web crawler project that

1. Extract the information from the 591 房屋交易網 item page e.g. [Example Page](https://newhouse.591.com.tw/110287).
2. Save information to your google cloud spreadsheet.

## Requirement

Go >= 1.17

## How to use

1. Set up environment variable in `.env` file.
   - Get your spreadsheet id by spreadsheet page url path. `https://docs.google.com/spreadsheets/d/<your spreadsheet id>/edit#gid=0`
   - Create a service account in GCP Console for this application with appropriate role, then download the json credential and set the path in `.env` file.
   - Add the service account email to your spreadsheet share list.
2. Run application by `go run main.go [urls]`, urls can be **one or more**
3. Information will append to your spreadsheet

## Dependencies

- google.golang.org/api
- github.com/joho/godotenv
- github.com/PuerkitoBio/goquery

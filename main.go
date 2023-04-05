package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	"google.golang.org/api/sheets/v4"
)

var (
	spreadsheetId = ""
	crawlDelay    = 500 * time.Millisecond
)

func init() {
	err := godotenv.Load("my.env")
	if err != nil {
		log.Fatalf("Unable to load .env file: %v", err)
	}

	spreadsheetId = os.Getenv("SPREADSHEET_ID")
}

func main() {
	fullData := make([][]interface{}, 0)
	ch := make(chan []interface{})
	argLen := len(os.Args)

	if argLen < 2 {
		log.Fatalf("Usage: go run main.go [urls]")
	}

	ctx := context.Background()
	client, err := sheets.NewService(ctx)
	if err != nil {
		log.Fatalf("Unable to create Google Sheets client: %v", err)
	}

	go func() {
		wg := &sync.WaitGroup{}
		i := 1
		for i < argLen {
			wg.Add(1)
			go crawl(strings.Split(os.Args[i], "?")[0], client, &ctx, wg, ch)
			time.Sleep(crawlDelay)
			i++
		}

		wg.Wait()
		close(ch)
	}()

	for d := range ch {
		fullData = append(fullData, d)
	}

	writeToSpreadsheet(client, fullData, &ctx)

}

func crawl(url string, client *sheets.Service, ctx *context.Context, wg *sync.WaitGroup, ch chan<- []interface{}) {
	data := make([]interface{}, 0)
	if len(strings.TrimSpace(url)) == 0 {
		fmt.Println("Skip: empty url")
	}

	detailUrl := url + "/detail"

	fmt.Println(url)

	mainDoc := getDocument(url)
	infoDoc := getDocument(detailUrl)

	data = append(data, findName(mainDoc)...)
	data = append(data, findUrl(mainDoc, url)...)
	data = append(data, findPricePerPing(mainDoc)...)
	data = append(data, findOtherInfo(mainDoc)...)
	data = append(data, findCommPlan(mainDoc)...)
	data = append(data, findSubDetail(infoDoc)...)

	wg.Done()

	ch <- data
}

func getDocument(url string) *goquery.Document {
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("Http request failed, err = %s", err)
	}
	defer res.Body.Close()
	mainDoc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatalf("Document parsing failed, err = %s", err)
	}

	return mainDoc
}

func findName(doc *goquery.Document) []interface{} {
	d := make([]interface{}, 0)
	title := doc.Find(".build-name").First().Text()
	return append(d, title)
}

func findUrl(doc *goquery.Document, url string) []interface{} {
	d := make([]interface{}, 0)
	return append(d, url)
}

func findPricePerPing(doc *goquery.Document) []interface{} {
	d := make([]interface{}, 0)
	pricePerPing := doc.Find(".build-price").ChildrenFiltered(".price").First().Text()
	return append(d, pricePerPing)
}

func findOtherInfo(doc *goquery.Document) []interface{} {
	d := make([]interface{}, 0)
	doc.Find(".intro-info").First().ChildrenFiltered(".info-item").Each(func(i int, s *goquery.Selection) {
		s.ChildrenFiltered("a[title]").First().Each(func(i int, s *goquery.Selection) {
			text, exist := s.Attr("title")
			href, _ := s.Attr("href")

			if exist && text == "基地地址" {
				d = append(d, fmt.Sprintf("=HYPERLINK(\"%s\", \"%s\")", "https:"+href, s.Text()))
			}
		})

		s.ChildrenFiltered("p").ChildrenFiltered(".value").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			d = append(d, text)
		})
	})

	return d
}

func findCommPlan(doc *goquery.Document) []interface{} {
	d := make([]interface{}, 0)
	doc.Find(".community-plan-container").Find(".list-item").Each(func(i int, s *goquery.Selection) {
		if i >= 4 && i <= 7 || i >= 12 {
			return
		}

		s.ChildrenFiltered("p").Each(func(i int, s *goquery.Selection) {
			d = append(d, s.Text())
		})

		s.ChildrenFiltered("div").Each(func(i int, s *goquery.Selection) {
			d = append(d, s.Text())
		})
	})

	return d
}

func findSubDetail(doc *goquery.Document) []interface{} {
	d := make([]interface{}, 0)
	doc.Find(".sub-detail-item.anchor-nav-item").Each(func(i int, s *goquery.Selection) {
		title := s.ChildrenFiltered("h3").First().Text()

		if title == "交通出行" || title == "周邊機能" {
			s.Find("li").Each(func(i int, s *goquery.Selection) {
				d = append(d, s.ChildrenFiltered("p").Text())
			})
		}
	})

	return d

}

func writeToSpreadsheet(client *sheets.Service, d [][]interface{}, ctx *context.Context) {
	body := &sheets.ValueRange{Values: d}
	_, err := client.Spreadsheets.Values.Append(spreadsheetId, "Main!A1:A1", body).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(*ctx).Do()
	if err != nil {
		log.Fatalf("Unable to append data to sheet: %v", err)
	}
}

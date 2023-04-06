package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	"google.golang.org/api/sheets/v4"
)

var (
	sheetName     = "Main"
	spreadsheetId = ""
	crawlDelay    = 500 * time.Millisecond
	headers       = []string{
		"標題",
		"連結",
		"價格/坪",
		"價格/戶",
		"車位價格",
		"格局",
		"坪數",
		"交屋時間",
		"建設公司",
		"地址",
		"公設比",
		"建蔽率",
		"基地面積",
		"管理費用",
		"車位配比",
		"車位規劃",
		"棟戶規劃",
		"樓層規劃",
		"捷運系統",
		"高速公路",
		"快速道路",
		"高鐵系統",
		"台鐵系統",
		"其他方式",
		"學區",
		"超商/賣場",
		"傳統市場",
		"公共建設",
		"熱門商圈",
		"醫療機構",
		"政府機構",
		"其他配套",
	}
)

func init() {
	err := godotenv.Load()
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

	makeHeader(client)

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

func makeHeader(client *sheets.Service) {
	res, err := client.Spreadsheets.Values.Get(spreadsheetId, fmt.Sprintf("%s!1:1", sheetName)).MajorDimension("ROWS").Do()
	if err != nil {
		log.Fatalf("Read spreadsheet failed. err = %v", err)
	}

	if len(res.Values) == 0 || !reflect.DeepEqual(res.Values[0], headers) {
		v := make([][]interface{}, 0)
		h := make([]interface{}, 0)
		for i := range headers {
			h = append(h, headers[i])
		}

		v = append(v, h)
		body := &sheets.ValueRange{Values: v}

		if len(res.Values) != 0 {
			_, err := client.Spreadsheets.Values.Clear(spreadsheetId, fmt.Sprintf("%s!1:1", sheetName), &sheets.ClearValuesRequest{}).Do()
			if err != nil {
				log.Fatalf("Clear spreadsheet header failed. err = %v", err)
			}
		}

		_, err = client.Spreadsheets.Values.Append(spreadsheetId, fmt.Sprintf("%s!1:1", sheetName), body).ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			log.Fatalf("Write spreadsheet header failed. err = %v", err)
		}
	}
}

func crawl(url string, client *sheets.Service, ctx *context.Context, wg *sync.WaitGroup, ch chan<- []interface{}) {
	data := make([]interface{}, 0)
	defer wg.Done()
	if len(strings.TrimSpace(url)) == 0 {
		fmt.Println("Skip: empty url")
		return
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

	ch <- data

}

func getDocument(url string) *goquery.Document {
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("Http request failed, err = %v", err)
	}
	defer res.Body.Close()
	mainDoc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatalf("Document parsing failed, err = %v", err)
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
	_, err := client.Spreadsheets.Values.Append(spreadsheetId, fmt.Sprintf("%s!A1:A1", sheetName), body).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(*ctx).Do()
	if err != nil {
		log.Fatalf("Unable to append data to sheet: %v", err)
	}
}

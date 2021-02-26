package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/schollz/progressbar/v3"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"time"
)

var colorReset = "\033[0m"
var colorGreen = "\033[32m"

type SendResult struct {
	WorkNum int
	Body string
	Code int
}

type CliArgs struct {
	Url string
	FileName string
	SheetName string
	Buffer int
}

func main() {
	fmt.Printf("Starting Go-Post -- An %sEXCEL%slent data sender!\n", colorGreen, colorReset)

	start := time.Now()
	cliArgs := getArgs()
	f, err := excelize.OpenFile(cliArgs.FileName)
	errHandler(err, "Failed to open Excel file.", true)

	rows, err := f.GetRows(cliArgs.SheetName)
	errHandler(err, "Failed to get rows from sheet.", true)

	qtyRows := len(rows) - 1
	bar := progressbar.Default(int64((qtyRows)*2))
	fmt.Printf("Found %d rows of data to send. Sending posts now, please wait.\n", qtyRows)

	var headers  []string
	for _, cell := range rows[0] {
		headers = append(headers, cell)
	}

	c := make(chan SendResult, cliArgs.Buffer)
	for i, row := range rows {
		if i == 0 { continue }
		bar.Add(1)
		go send(row, headers, cliArgs.Url, i, c)
	}

	var failed []SendResult
	ok := 0
	for i := 0; i < qtyRows; i++ {
		r := <- c
		if r.Code < 200 || r.Code > 299 {
			failed = append(failed, r)
		} else {
			ok++
		}
		bar.Add(1)
	}

	qtyFailed := len(failed)
	if qtyFailed > 0 {
		fmt.Println("The following Excel lines:")
		for _, r := range failed {
			fmt.Printf("Excel line: %d | Status code: %d | Error: %s\n", r.WorkNum, r.Code, r.Body)
		}
		fmt.Println()
	}

	sr := math.Floor(float64(ok) / float64(qtyRows) * 100.0)
	fmt.Printf("All done! Success rate was %.2f%% and elapsed time was %s\n", sr, time.Since(start))
	os.Exit(0)
}

func errHandler(err error, msg string, fatal bool)  {
	if err == nil {
		return
	}
	fmt.Println(msg, err)
	if fatal {
		os.Exit(-1)
	}
}

func getArgs() CliArgs {
	urlPt := flag.String("url", "", "Url to be used in the posts.")
	filePt := flag.String("file", "", "Path to the Excel file containing the payload.")
	sheetPt := flag.String("sheet", "Sheet1", "Name of the sheet containing the data we'll use.")
	bufferPt := flag.Int("buffer", 2, "Size of the worker buffer.")
	flag.Parse()
	if *urlPt == "" || *filePt == "" {
		fmt.Println("Malformed command line arguments. Use 'go-post --help' to check the correct usage.")
		os.Exit(-1)
	}

	return CliArgs{
		Url:      *urlPt,
		FileName: *filePt,
		Buffer:   *bufferPt,
		SheetName: *sheetPt,
	}
}

func getErrStatusCode(resp *http.Response) int {
	var statusCode int
	if resp != nil {
		statusCode = resp.StatusCode
	} else {
		statusCode = 0
	}
	return statusCode
}

func send(row []string, headers []string, url string, worker int, c chan SendResult) {
	payload := make(map[string] string)
	for i, header := range headers {
		payload[header] = row[i]
	}
	jsonString, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonString))
	if err != nil {
		c <- SendResult {
			WorkNum: worker,
			Body: fmt.Sprint("Failed to create request.", err),
			Code: 0,
		}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c <- SendResult {
			WorkNum: worker,
			Body: fmt.Sprint("Request failed.", err),
			Code: getErrStatusCode(resp),
		}
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c <- SendResult {
			WorkNum: worker,
			Body: fmt.Sprint("Failed to read response body.", err),
			Code: resp.StatusCode,
		}
		return
	}

	c <- SendResult {
		WorkNum: worker,
		Body: string(body),
		Code: resp.StatusCode,
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type PostSendResult struct {
	WorkNum int
	Body string
	Code int
}

type goPostCliArgs struct {
	Url string
	FileName string
	SheetName string
	Buffer int
}

func main() {
	fmt.Printf("Starting Go-Post -- An %sEXCEL%slent data sender!\n", ColorReset, ColorReset)

	start := time.Now()
	cliArgs := getGoPostArgs()
	rows, qtyRows := ReadExcel(cliArgs.FileName, cliArgs.SheetName)

	bar := progressbar.Default(int64((qtyRows)*2))
	fmt.Printf("Found %d rows of data to send. Sending posts now, please wait.\n", qtyRows)

	var headers  []string
	for _, cell := range rows[0] {
		headers = append(headers, cell)
	}

	c := make(chan PostSendResult, cliArgs.Buffer)
	for i, row := range rows {
		if i == 0 { continue }
		bar.Add(1)
		go send(row, headers, cliArgs.Url, i, c)
	}

	var failed []PostSendResult
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

	PrintDoneMessage(ok, qtyRows, start)
	os.Exit(0)
}

func send(row []string, headers []string, url string, worker int, c chan PostSendResult) {
	payload := make(map[string] string)
	for i, header := range headers {
		payload[header] = row[i]
	}
	jsonString, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonString))
	if err != nil {
		c <- PostSendResult{
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
		c <- PostSendResult{
			WorkNum: worker,
			Body: fmt.Sprint("Request failed.", err),
			Code: getErrStatusCode(resp),
		}
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c <- PostSendResult{
			WorkNum: worker,
			Body: fmt.Sprint("Failed to read response body.", err),
			Code: resp.StatusCode,
		}
		return
	}

	c <- PostSendResult{
		WorkNum: worker,
		Body: string(body),
		Code: resp.StatusCode,
	}
}

func getGoPostArgs() goPostCliArgs {
	urlPt := flag.String("url", "", "Url to be used in the posts.")
	filePt := flag.String("file", "", "Path to the Excel file containing the payload.")
	sheetPt := flag.String("sheet", "Sheet1", "Name of the sheet containing the data we'll use.")
	bufferPt := flag.Int("buffer", 2, "Size of the worker buffer.")
	flag.Parse()
	if *urlPt == "" || *filePt == "" {
		fmt.Println("Malformed command line arguments. Use 'go-post --help' to check the correct usage.")
		os.Exit(-1)
	}

	return goPostCliArgs{
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
package main

import (
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"
)

var ColorReset = "\033[0m"
var ColorGreen = "\033[32m"

func ReadExcel(file string, sheet string) ([][]string, int) {
	f, err := excelize.OpenFile(file)
	ErrorHandler(err, "Failed to open Excel file.", true)

	rows, err := f.GetRows(sheet)
	ErrorHandler(err, "Failed to get rows from sheet.", true)

	qtyRows := len(rows) - 1

	return rows, qtyRows
}

func GetFiles(dir string) ([]string, int) {
	files, err := ioutil.ReadDir(dir)
	var filenames []string
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		filenames = append(filenames, filepath.Join(dir, file.Name()))
	}

	return filenames, len(files)
}

func ReadTextFile(f string) (string, error) {
	content, err := ioutil.ReadFile(f)
	if err != nil {
		return "", err
	}
	return string(content), err
}

func PrintDoneMessage(qtyOk int, total int, start time.Time) {
	sr := math.Floor(float64(qtyOk) / float64(total) * 100.0)
	fmt.Printf("All done! Success rate was %.2f%% and elapsed time was %s\n", sr, time.Since(start))
}

func ErrorHandler(err error, msg string, fatal bool)  {
	if err == nil {
		return
	}
	fmt.Println(msg, err)
	if fatal {
		os.Exit(-1)
	}
}
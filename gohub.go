package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Azure/azure-event-hubs-go/v3"
	"github.com/schollz/progressbar/v3"
	"os"
	"time"
)

type eventHubSendResult struct {
	FileIndex int
	Success bool
	Error error
}

type goHubCliArgs struct {
	PayloadPath string
	ConnectionString string
	Buffer int
}


func main() {
	fmt.Printf("Starting Go-Hub -- An %sEXCEL%slent data sender!\n", ColorGreen, ColorReset)
	start := time.Now()
	cliArgs := getGoHubArgs()
	hub := getEventHub(cliArgs.ConnectionString)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	payloadFiles, qtyFiles := GetFiles(cliArgs.PayloadPath)
	bar := progressbar.Default(int64((qtyFiles)*2))
	c := make(chan eventHubSendResult, cliArgs.Buffer)
	for i, file := range payloadFiles {
		bar.Add(1)
		go sendMessage(hub, ctx, file, c, i)
	}

	var failed []eventHubSendResult
	ok := 0
	for i := 0; i < qtyFiles; i++ {
		r := <- c
		if r.Success {
			ok++
		} else {
			failed = append(failed, r)
		}
		bar.Add(1)
	}

	qtyFailed := len(failed)
	if qtyFailed > 0 {
		fmt.Println("The following files failed:")
		for _, r := range failed {
			fmt.Printf("File: %s  | Error: %s\n", payloadFiles[r.FileIndex], r.Error)
		}
		fmt.Println()
	}

	PrintDoneMessage(ok, qtyFiles, start)
	os.Exit(0)
}

func sendMessage(hub *eventhub.Hub, ctx context.Context, file string, c chan eventHubSendResult, i int) {
	content, err := ReadTextFile(file)
	if err != nil {
		c <- eventHubSendResult{
			FileIndex: i,
			Success:   false,
			Error:     err,
		}
		return
	}

	err = hub.Send(ctx, eventhub.NewEventFromString(content))
	c <- eventHubSendResult{
		FileIndex: i,
		Success:   err == nil,
		Error:     err,
	}
}

func getEventHub(cs string) *eventhub.Hub {
	hub, err := eventhub.NewHubFromConnectionString(cs)
	if err != nil {
		panic(err)
	}
	return hub
}

func getGoHubArgs() goHubCliArgs {
	pathPt := flag.String("path", "", "Path where the payload files are.")
	connStrPt := flag.String("connection-string", "$file$", "Azure eventhub connection string. Must include HubName. If omitted, will try to read it from eventhub.conn.txt file.")
	bufferPt := flag.Int("buffer", 2, "Size of the worker buffer.")
	flag.Parse()
	if *pathPt == "" {
		fmt.Println("Malformed command line arguments. Use 'go-post --help' to check the correct usage.")
		os.Exit(-1)
	}
	var connStr string
	var err error
	if *connStrPt == "$file$" {
		connStr, err = ReadTextFile("eventhub.conn.txt")
		ErrorHandler(err, "Failed to get connection string from file.", true)
	} else {
		connStr = *connStrPt
	}

	return goHubCliArgs{
		PayloadPath: *pathPt,
		Buffer:   *bufferPt,
		ConnectionString: connStr,
	}
}
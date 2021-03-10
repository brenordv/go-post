package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"strings"
	"time"
)

type reqResult struct {
	WorkerNum int
	DocsFound int
	Elapsed time.Duration
}

type GoMongoCliArgs struct {
	ConnectionString string
	Database string
	Collection string
	Query string
	Buffer int
	Delay int
	NumReq int
	SingleConn bool
}

func main() {
	args := getGoMongoArgs()

	var preparedQuery bson.M
	err := json. Unmarshal([]byte(args.Query), &preparedQuery)
	ErrorHandler(
		err,
		fmt.Sprintf("Failed to prepare query '%s' to be used as filter in mongo find.", args.Query),
		true)

	clientOptions := options. Client(). ApplyURI(args.ConnectionString)

	if args.SingleConn {
		stressSingleConnection(
			clientOptions,
			args.Database,
			args.Collection,
			preparedQuery,
			args.Buffer,
			args.Delay,
			args.NumReq)
	} else {
		stressMultiConnection(
			clientOptions,
			args.Database,
			args.Collection,
			preparedQuery,
			args.Buffer,
			args.Delay,
			args.NumReq)
	}
}

func stressSingleConnection(clientOptions *options.ClientOptions, database string, collection string, query bson.M, buffer int, delayMs int, numReqs int) {
	c := make(chan reqResult, buffer)
	defer close(c)
	client, ctx := getClient(clientOptions, 0)
	defer client.Disconnect(ctx)

	col := client.Database(database).Collection(collection)
	bar := progressbar.Default(int64((numReqs)*2))
	for i := 0; i < numReqs; i++ {
		bar.Add(1)
		go reqUsingConn(query, col, ctx, i, c)
		delay(delayMs)
	}

	var results []reqResult
	for i := 0; i < numReqs; i++ {
		results = append(results, <-c)
		bar.Add(1)

	}

	printResults(results)
}


func stressMultiConnection(clientOptions *options.ClientOptions, database string, collection string, query bson.M, buffer int, delayMs int, numReqs int) {
	c := make(chan reqResult, buffer)
	defer close(c)

	bar := progressbar.Default(int64((numReqs)*2))
	for i := 0; i < numReqs; i++ {
		bar.Add(1)
		go singleRequest(clientOptions, database, collection, query, i, c)
		delay(delayMs)
	}

	var results []reqResult
	for i := 0; i < numReqs; i++ {
		results = append(results, <-c)
		bar.Add(1)

	}
	printResults(results)
}

func reqUsingConn(query bson.M, col *mongo.Collection, ctx context.Context, workerNum int, c chan reqResult) {
	start := time.Now()

	var err error
	var cursor *mongo.Cursor
	cursor, err = col.Find(ctx, query)
	ErrorHandler(err, fmt.Sprintf("[W{%d}] Error fetching from MongoDB!", workerNum), true)

	c <- reqResult{
		WorkerNum: workerNum,
		DocsFound: cursor.RemainingBatchLength(),
		Elapsed:   time.Since(start),
	}

	cursor.Close(ctx)
}

func singleRequest(clientOptions *options.ClientOptions, database string, collection string, query bson.M, workerNum int, c chan reqResult) {
	start := time.Now()
	client, ctx := getClient(clientOptions, workerNum)
	defer client.Disconnect(ctx)

	col := client.Database(database).Collection(collection)
	cursor, err := col.Find(ctx, query)
	ErrorHandler(err, fmt.Sprintf("[W{%d}] Error fetching from MongoDB!", workerNum), true)

	c <- reqResult{
		WorkerNum: workerNum,
		DocsFound: cursor.RemainingBatchLength(),
		Elapsed:   time.Since(start),
	}
}

func ping(c *mongo.Client, ctx context.Context, workerNum int) {
	err := c.Ping(ctx, readpref.Primary())
	ErrorHandler(err, fmt.Sprintf("[W{%d}] Connected, but couldnt ping!", workerNum), true)
}

func getClient(clientOptions *options.ClientOptions, workerNum int) (*mongo.Client, context.Context) {
	client, err := mongo.Connect(context.TODO(), clientOptions)
	ErrorHandler(err, fmt.Sprintf("[W{%d}] Connection Error!", workerNum), true)

	ctx := context.Background()
	ping(client, ctx, workerNum)

	return client, ctx
}

func printResults(results []reqResult) {
	var sumDuration int64
	for _, res := range results {
		sumDuration += res.Elapsed.Nanoseconds()
		fmt.Printf("Worker %d found %d docs in %s.\n",  res.WorkerNum, res.DocsFound, res.Elapsed)
	}
	var avg = sumDuration / int64(len(results))
	fmt.Println("Average time spent on each request:", time.Duration(avg))
}

func delay(d int) {
	if d == 0 { return }
	time.Sleep(time.Duration(d)  * time.Millisecond)
}

func getGoMongoArgs() GoMongoCliArgs {
	connStrPt := flag.String("connection-string", "$file$", "Azure eventhub connection string. Must include HubName. If omitted, will try to read it from mongodb.conn.txt file.")
	dbPt := flag.String("database", "admin", "Name of the database that will be used.")
	clPt := flag.String("collection", "", "Name of the collection that will be used.")
	queryPt := flag.String("query", "", "Query that will be used when trying to find objects.")
	bufferPt := flag.Int("buffer", 2, "Size of the worker buffer.")
	delayPt := flag.Int("delay", 0, "Delay between requests.")
	numReqsPt := flag.Int("requests", 100, "Number of requests that will be made.")
	singleConnPt := flag.Bool("single-conn", false, "If used, will share a single connection through all requests.")
	flag.Parse()

	query := *queryPt
	if strings.Trim(query, " ") == "" {
		query = "{}"
	}

	var connStr string
	var err error
	if *connStrPt == "$file$" {
		connStr, err = ReadTextFile("mongodb.conn.txt")
		ErrorHandler(err, "Failed to get connection string from file.", true)
	} else {
		connStr = *connStrPt
	}

	return GoMongoCliArgs{
		ConnectionString: connStr,
		Database:         *dbPt,
		Collection:       *clPt,
		Query:            query,
		Buffer:           *bufferPt,
		Delay:            *delayPt,
		NumReq:           *numReqsPt,
		SingleConn:       *singleConnPt,
	}
}

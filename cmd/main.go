package main

import (
	"fmt"
	"github.com/apache/iotdb-client-go/client"
)

func main() {
	clientConfig := client.Config{
		Host:     "127.0.0.1",
		Port:     "6667",
		UserName: "root",
		Password: "root",
	}
	timeoutInMs := int64(10)

	session := client.NewSession(&clientConfig)
	session.Open(false, 10)

	session.ExecuteStatement("CREATE DATABASE root.sg")

	session.ExecuteStatement("CREATE TIMESERIES root.sg.d1.s1 WITH DATATYPE=INT64, ENCODING=RLE")
	session.ExecuteStatement("CREATE TIMESERIES root.sg.d1.s2 WITH DATATYPE=INT64, ENCODING=RLE")
	session.ExecuteStatement("CREATE TIMESERIES root.sg.d1.s3 WITH DATATYPE=INT64, ENCODING=RLE")

	session.ExecuteStatement("INSERT INTO root.sg.d1(timestamp, s1, s2, s3) VALUES (1, 1, 2, 3), (2, 10, 11, 12)")

	var dataSet *client.SessionDataSet
	dataSet, _ = session.ExecuteQueryStatement("SELECT * FROM root.sg.**", &timeoutInMs)

	for next, err := dataSet.Next(); err == nil && next; next, err = dataSet.Next() {
		fmt.Printf("%s\t", dataSet.GetText("Time"))

		for i := 0; i < dataSet.GetColumnCount(); i++ {
			columnName := dataSet.GetColumnName(i)
			fmt.Printf("%v\t", columnName)
			v := dataSet.GetValue(columnName)
			fmt.Printf("%v\t\t", v)
		}
		fmt.Println()
	}
}

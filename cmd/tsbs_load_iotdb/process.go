package main

import (
	"fmt"
	"github.com/apache/iotdb-client-go/client"
	"github.com/apache/iotdb-client-go/common"
	"github.com/benchant/tsbs/pkg/targets"
	"github.com/benchant/tsbs/pkg/targets/iotdb"
	"os"
	"strconv"
	"strings"
)

type processor struct {
	numWorker                int // the worker(like thread) ID of this processor
	session                  client.Session
	recordsMaxRows           int // max rows of records in 'InsertRecords'
	tabletSize               int
	ProcessedTagsDeviceIDMap map[string]bool // already processed device ID
	tabletsMap               map[string]*client.Tablet

	loadToSCV         bool                // if true, do NOT insert into databases, but generate csv files instead.
	csvFilepathPrefix string              // Prefix of filepath for csv files. Specific a folder or a folder with filename prefix.
	filePtrMap        map[string]*os.File // file pointer for each deviceID

	useAlignedTimeseries bool // using aligned timeseries if set true.
	useInsertRecords     bool
	storeTags            bool // store tags if set true. Can NOT be used if useAlignedTimeseries is set true.
}

func (p *processor) Init(numWorker int, doLoad, hashWorkers bool) {
	p.numWorker = numWorker
	if !doLoad {
		return
	}
	if p.loadToSCV {
		p.filePtrMap = make(map[string]*os.File)
	} else {
		p.ProcessedTagsDeviceIDMap = make(map[string]bool)
		p.tabletsMap = make(map[string]*client.Tablet)
		p.session = client.NewSession(&clientConfig)
		if err := p.session.Open(false, timeoutInMs); err != nil {
			errMsg := fmt.Sprintf("IoTDB processor init error, session is not open: %v, ", err)
			errMsg = errMsg + fmt.Sprintf("timeout setting: %d ms\n", timeoutInMs)
			fatal(errMsg)
		}

		sql := "create device template r1 aligned (latitude DOUBLE, longitude DOUBLE, elevation INT32, velocity INT32, heading INT32, grade INT32, fuel_consumption DOUBLE);"
		_, err := p.session.ExecuteStatement(sql)
		if err != nil {
			fatal("ExecuteStatement create device template r1 error: %v", err)
		}
		sql = "create database root.readings;"
		_, err = p.session.ExecuteStatement(sql)
		if err != nil {
			fatal("ExecuteStatement create database root.readings error: %v", err)
		}
		sql = "set DEVICE TEMPLATE r1 to root.readings;"
		_, err = p.session.ExecuteStatement(sql)
		if err != nil {
			fatal("ExecuteStatement set DEVICE TEMPLATE r1 error: %v", err)
		}

		sql = "create device template d1 aligned (fuel_state DOUBLE, current_load INT32, status INT32);"
		_, err = p.session.ExecuteStatement(sql)
		if err != nil {
			fatal("ExecuteStatement create device template r1 error: %v", err)
		}
		sql = "create database root.diagnostics;"
		_, err = p.session.ExecuteStatement(sql)
		if err != nil {
			fatal("ExecuteStatement create database root.diagnostics error: %v", err)
		}
		sql = "set DEVICE TEMPLATE r1 to root.diagnostics;"
		_, err = p.session.ExecuteStatement(sql)
		if err != nil {
			fatal("ExecuteStatement set DEVICE TEMPLATE d1 error: %v", err)
		}
	}
}

type records struct {
	deviceIds    []string
	measurements [][]string
	dataTypes    [][]client.TSDataType
	values       [][]interface{}
	timestamps   []int64
}

func (p *processor) ProcessBatch(b targets.Batch, doLoad bool) (metricCount, rowCount uint64) {
	batch := b.(*iotdbBatch)

	if !doLoad {
		return batch.metricsCnt, uint64(batch.rowCnt)
	}

	if p.loadToSCV {
		// TODO add load csv impl
		return 0, 0
	}

	// using `insertRecords` API
	if p.tabletSize <= 0 {
		var rcds records
		for device, values := range batch.m {

			db := strings.Split(device, ".")[0]
			fullDevice := "root." + device

			for _, value := range values {
				splits := strings.Split(value, ",")
				if splits[0] == "tag" {
					if !p.storeTags {
						continue
					}
					kvString := splits[1]
					for i, kv := range splits {
						if i > 1 {
							kvString = kvString + "," + kv
						}
					}
					sql := fmt.Sprintf("CREATE ALIGNED TIMESERIES %s(_tags INT32 tags(%s)) ", fullDevice, kvString)
					_, err := p.session.ExecuteStatement(sql)
					if err != nil {
						fatal("ExecuteStatement CREATE timeseries with tags error: %v", err)
					}
					continue
				}

				rcds.deviceIds = append(rcds.deviceIds, fullDevice)
				rcds.measurements = append(rcds.measurements, iotdb.GlobalMeasurementMap[db])
				dataTypes := iotdb.GlobalDataTypeMap[db]
				rcds.dataTypes = append(rcds.dataTypes, dataTypes)

				timestamp, err := strconv.ParseInt(splits[0], 10, 64)
				if err != nil {
					fatal("parse timestamp error: %d, %s", timestamp, err)
				}
				rcds.timestamps = append(rcds.timestamps, timestamp)

				var valueList []interface{}
				for cIdx, v := range splits[1:] {
					nv, err := parseDataToInterface(dataTypes[cIdx], v)
					if err != nil {
						fatal("parse data value error: %d, %s", v, err)
					}
					valueList = append(valueList, nv)
				}

				rcds.values = append(rcds.values, valueList)
			}
		}

		var s *common.TSStatus
		var err error
		if p.useAlignedTimeseries {
			s, err = p.session.InsertAlignedRecords(rcds.deviceIds, rcds.measurements, rcds.dataTypes, rcds.values, rcds.timestamps)
		} else {
			s, err = p.session.InsertRecords(rcds.deviceIds, rcds.measurements, rcds.dataTypes, rcds.values, rcds.timestamps)
		}

		if err != nil {
			fatal("Invoking Insert Records API meets error: %v", err)
		}
		if s.Code != client.SuccessStatus {
			fatal("Invoking Insert Records API returns failure status, code: %v, message: %v", s.Code, s.GetMessage())
		}

		metricCount = batch.metricsCnt
		rowCount = uint64(batch.rowCnt)
		batch.Reset()
		return metricCount, rowCount
	}

	for dbTruckName, values := range batch.m {
		segments := strings.Split(dbTruckName, ".")
		db := segments[0]
		truckName := segments[1]
		dataTypes := iotdb.GlobalDataTypeMap[db]

		var tablet *client.Tablet
		var err error
		var exist bool

		fullTruckPath, fullTruckPathExist := iotdb.GlobalTruckNameWithPath[dbTruckName]
		if fullTruckPathExist {
			tablet, exist = p.tabletsMap[fullTruckPath]

			if !exist {
				tablet, err = client.NewTablet(fullTruckPath, iotdb.GlobalTabletSchemaMap[db], p.tabletSize)
				if err != nil {
					fatal("build tablet error: %s", err)
				}
				p.tabletsMap[fullTruckPath] = tablet
			}
		}

		for _, value := range values {
			splits := strings.Split(value, ",")

			if splits[0] == "tag" {
				if !p.storeTags {
					continue
				}

				// kvString := splits[1]
				tmpFullTruckPath := ""
				attribute := ""
				fleet := ""
				model := ""
				driver := ""
				for i, kv := range splits {
					if i == 0 || i == 1 {
						continue
					}

					arrays := strings.Split(kv, "=")
					if arrays[0] == iotdb.Fleet {
						fleet = arrays[1]
					} else if arrays[0] == iotdb.Model {
						model = arrays[1]
					} else if arrays[0] == iotdb.Driver {
						driver = arrays[1]
					} else if arrays[0] == iotdb.NominalFuelConsumption || arrays[0] == iotdb.DeviceVersion ||
						arrays[0] == iotdb.LoadCapacity || arrays[0] == iotdb.FuelCapacity {
						if attribute == "" {
							attribute = kv
						} else {
							attribute += "," + kv
						}
					}

					// kvString = kvString + "," + kv
				}

				tmpFullTruckPath = "root." + db + "." + fleet + "." + model + "." + truckName + "." + driver
				iotdb.GlobalTruckNameWithPath[dbTruckName] = tmpFullTruckPath
				tablet, err = client.NewTablet(tmpFullTruckPath, iotdb.GlobalTabletSchemaMap[db], p.tabletSize)
				p.tabletsMap[tmpFullTruckPath] = tablet
				if err != nil {
					fatal("build tablet error: %s", err)
				}

				// TODO add template impl
				seriesCreateSql := ""
				if db == iotdb.Readings {
					seriesCreateSql = fmt.Sprintf("create timeseries using DEVICE TEMPLATE r1 on %s) ", tmpFullTruckPath)
				} else {
					seriesCreateSql = fmt.Sprintf("create timeseries using DEVICE TEMPLATE d1 on %s) ", tmpFullTruckPath)
				}

				_, err = p.session.ExecuteStatement(seriesCreateSql)
				if err != nil {
					fatal("ExecuteStatement CREATE timeseries with tags error: %v", err)
				}

				sql := fmt.Sprintf("CREATE TIMESERIES root.attr.%s._attributes INT32 attributes(%s)", truckName, attribute)
				_, err = p.session.ExecuteStatement(sql)
				if err != nil {
					fatal("ExecuteStatement CREATE timeseries with tags error: %v", err)
				}

				continue
			}

			timestamp, err := strconv.ParseInt(splits[0], 10, 64)
			if err != nil {
				fatal("parse timestamp error: %d, %s", timestamp, err)
			}

			tablet.SetTimestamp(timestamp, tablet.RowSize)

			for cIdx, v := range splits[1:] {
				// TODO perfect deal with null value
				nv, err := parseDataToInterface(dataTypes[cIdx], v)
				if err != nil {
					fatal("Parse data value error: %d, %s", v, err)
				}

				err = tablet.SetValueAt(nv, cIdx, tablet.RowSize)
				if err != nil {
					fatal("InsertTablet SetValueAt error: %v", err)
				}
			}

			tablet.RowSize += 1

			if tablet.RowSize >= p.tabletSize {
				var r *common.TSStatus
				var err error
				if p.useAlignedTimeseries {
					r, err = p.session.InsertAlignedTablet(tablet, true)
				} else {
					r, err = p.session.InsertTablet(tablet, true)
				}
				if err != nil {
					fatal("InsertTablet meets error: %v", err)
				}
				if r.Code != client.SuccessStatus {
					fatal("InsertTablet meets error for status is not equals Success: %v, %v", r, r.GetMessage())
				}

				tablet.Reset()
			}
		}
	}

	metricCount = batch.metricsCnt
	rowCount = uint64(batch.rowCnt)
	batch.Reset()
	return metricCount, rowCount
}

// parse datatype and convert string into interface
func parseDataToInterface(datatype client.TSDataType, str string) (interface{}, error) {
	switch datatype {
	case client.INT32:
		if str == "" {
			return interface{}(int32(0)), nil
		}
		value, err := strconv.ParseInt(str, 10, 32)
		return interface{}(int32(value)), err
	case client.INT64:
		value, err := strconv.ParseInt(str, 10, 64)
		return interface{}(value), err
	case client.DOUBLE:
		if str == "" {
			return interface{}(0.0), nil
		}
		value, err := strconv.ParseFloat(str, 64)
		return interface{}(value), err
	default:
		return interface{}(nil), fmt.Errorf("unknown datatype, value:%s", str)
	}
}

func (p *processor) Close(_ bool) {
	for _, tablet := range p.tabletsMap {
		if tablet.Len() > 0 {
			var r *common.TSStatus
			var err error
			if p.useAlignedTimeseries {
				r, err = p.session.InsertAlignedTablet(tablet, true)
			} else {
				r, err = p.session.InsertTablet(tablet, true)
			}
			if err != nil {
				fatal("InsertTablet meets error: %v", err)
			}
			if r.Code != client.SuccessStatus {
				fatal("InsertTablet meets error for status is not equals Success: %v, %v", r, r.GetMessage())
			}
			tablet.Reset()
		}
	}
	defer p.session.Close()
}

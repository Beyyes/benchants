package iotdb

import (
	"github.com/apache/iotdb-client-go/client"
)

var (
	GlobalIoTTabletSchemaMap = make(map[string][]*client.MeasurementSchema)
	GlobalIoTMeasurementMap  = make(map[string][]string)
	GlobalIoTDataTypeMap     = make(map[string][]client.TSDataType)
	GlobalTruckNameWithPath  = make(map[string]string)

	// path
	Fleet  = "fleet"
	Model  = "model"
	Name   = "name"
	Driver = "driver"

	// attr
	NominalFuelConsumption = "nominal_fuel_consumption"
	DeviceVersion          = "device_version"
	LoadCapacity           = "load_capacity"
	FuelCapacity           = "fuel_capacity"

	Readings        = "readings"
	latitude        = "latitude"
	longitude       = "longitude"
	elevation       = "elevation"
	velocity        = "velocity"
	heading         = "heading"
	grade           = "grade"
	fuelConsumption = "fuel_consumption"

	diagnostics = "diagnostics"
	fuelState   = "fuel_state"
	currentLoad = "current_load"
	status      = "status"
)

func init() {
	GlobalMeasurementMap[Readings] = append(GlobalMeasurementMap[Readings], latitude, longitude, elevation,
		velocity, heading, grade, fuelConsumption)
	GlobalDataTypeMap[Readings] = append(GlobalDataTypeMap[Readings], client.DOUBLE, client.DOUBLE,
		client.INT32, client.INT32, client.INT32, client.INT32, client.DOUBLE)
	GlobalTabletSchemaMap[Readings] = append(GlobalTabletSchemaMap[Readings],
		&client.MeasurementSchema{Measurement: latitude, DataType: client.DOUBLE},
		&client.MeasurementSchema{Measurement: longitude, DataType: client.DOUBLE},
		&client.MeasurementSchema{Measurement: elevation, DataType: client.INT32},
		&client.MeasurementSchema{Measurement: velocity, DataType: client.INT32},
		&client.MeasurementSchema{Measurement: heading, DataType: client.INT32},
		&client.MeasurementSchema{Measurement: grade, DataType: client.INT32},
		&client.MeasurementSchema{Measurement: fuelConsumption, DataType: client.DOUBLE})

	GlobalMeasurementMap[diagnostics] = append(GlobalMeasurementMap[diagnostics], fuelState, currentLoad, status)
	GlobalDataTypeMap[diagnostics] = append(GlobalDataTypeMap[diagnostics], client.DOUBLE, client.INT32, client.INT32)
	GlobalTabletSchemaMap[diagnostics] = append(GlobalTabletSchemaMap[diagnostics],
		&client.MeasurementSchema{Measurement: fuelState, DataType: client.DOUBLE},
		&client.MeasurementSchema{Measurement: currentLoad, DataType: client.INT32},
		&client.MeasurementSchema{Measurement: status, DataType: client.INT32})

}

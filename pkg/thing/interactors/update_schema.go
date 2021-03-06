package interactors

import (
	"fmt"

	"github.com/CESARBR/knot-babeltower/pkg/thing/entities"
	"github.com/go-playground/validator"
)

type schemaType struct {
	valueType interface{}
	unit      interface{}
}

type interval struct {
	min int
	max int
}

// rules reference table: https://knot-devel.cesar.org.br/doc/thing/unit-type-value.html
var rules = map[int]schemaType{
	0x0000: {valueType: interval{1,7}, unit: 0},              // NONE
	0x0001: {valueType: interval{1,7}, unit: interval{1, 3}}, // VOLTAGE
	0x0002: {valueType: interval{1,7}, unit: interval{1, 2}}, // CURRENT
	0x0003: {valueType: interval{1,7}, unit: 1},              // RESISTENCE
	0x0004: {valueType: interval{1,7}, unit: interval{1, 3}}, // POWER
	0x0005: {valueType: interval{1,7}, unit: interval{1, 3}}, // TEMPERATURE
	0x0006: {valueType: interval{1,7}, unit: 1},              // RELATIVE_HUMIDITY
	0x0007: {valueType: interval{1,7}, unit: interval{1, 3}}, // LUMINOSITY
	0x0008: {valueType: interval{1,7}, unit: interval{1, 3}}, // TIME
	0x0009: {valueType: interval{1,7}, unit: interval{1, 4}}, // MASS
	0x000A: {valueType: interval{1,7}, unit: interval{1, 3}}, // PRESSURE
	0x000B: {valueType: interval{1,7}, unit: interval{1, 4}}, // DISTANCE
	0x000C: {valueType: interval{1,7}, unit: interval{1, 2}}, // ANGLE
	0x000D: {valueType: interval{1,7}, unit: interval{1, 4}}, // VOLUME
	0x000E: {valueType: interval{1,7}, unit: interval{1, 3}}, // AREA
	0x000F: {valueType: interval{1,7}, unit: 1},              // RAIN
	0x0010: {valueType: interval{1,7}, unit: 1},              // DENSITY
	0x0011: {valueType: interval{1,7}, unit: 1},              // LATITUDE
	0x0012: {valueType: interval{1,7}, unit: 1},              // LONGITUDE
	0x0013: {valueType: interval{1,7}, unit: interval{1, 4}}, // SPEED
	0x0014: {valueType: interval{1,7}, unit: interval{1, 6}}, // VOLUMEFLOW
	0x0015: {valueType: interval{1,7}, unit: interval{1, 6}}, // ENERGY
	0xFFF0: {valueType: interval{1,7}, unit: 0},              // PRESENCE
	0xFFF1: {valueType: interval{1,7}, unit: 0},              // SWITCH
	0xFFF2: {valueType: interval{1,7}, unit: 0},              // COMMAND
	0xFF10: {valueType: interval{1,7}, unit: 0},              // GENERIC
	0xFFFF: {valueType: interval{1,7}, unit: 0},              // INVALID
}

// UpdateSchema receive the new sensor schema and update it on the thing's service
func (i *ThingInteractor) UpdateSchema(authorization, thingID string, schemaList []entities.Schema) error {
	if authorization == "" {
		return ErrAuthNotProvided
	}
	if thingID == "" {
		return ErrIDNotProvided
	}
	if schemaList == nil {
		return ErrSchemaNotProvided
	}

	if !i.isValidSchema(schemaList) {
		err := i.notifyClient(thingID, schemaList, ErrSchemaInvalid)
		return err
	}
	i.logger.Info("updateSchema: schema validated")

	err := i.thingProxy.UpdateSchema(authorization, thingID, schemaList)
	if err != nil {
		sendErr := i.notifyClient(thingID, schemaList, err)
		return sendErr
	}
	i.logger.Info("updateSchema: schema updated")

	err = i.notifyClient(thingID, schemaList, err)
	if err != nil {
		// TODO: handle error when publishing message to queue.
		return err
	}
	i.logger.Info("updateSchema: message sent to client")

	return nil
}

func (i *ThingInteractor) isValidSchema(schemaList []entities.Schema) bool {
	validate := validator.New()
	validate.RegisterStructValidation(schemaValidation, entities.Schema{})
	for _, schema := range schemaList {
		err := validate.Struct(schema)
		if err != nil {
			return false
		}
	}

	return true
}

func (i *ThingInteractor) notifyClient(thingID string, schemaList []entities.Schema, err error) error {
	sendErr := i.publisher.PublishUpdatedSchema(thingID, schemaList, err)
	if sendErr != nil {
		if err != nil {
			return fmt.Errorf("error sending response to client: %v: %w", sendErr, err)
		}
		return fmt.Errorf("error sending response to client: %w", sendErr)
	}
	return err
}

func schemaValidation(sl validator.StructLevel) {
	schema := sl.Current().Interface().(entities.Schema)
	typeID := schema.TypeID

	if (typeID < 0 || 15 < typeID) && (typeID < 0xfff0 || 0xfff2 < typeID) && typeID != 0xff10 {
		sl.ReportError(schema, "schema", "Type ID", "typeID", "false")
		return
	}

	if !isValidValueType(schema.TypeID, schema.ValueType) {
		sl.ReportError(schema, "schema", "Value Type", "valueType", "false")
		return
	}

	if !isValidUnit(schema.TypeID, schema.Unit) {
		sl.ReportError(schema, "schema", "Unit", "unit", "false")
	}
}

func isValidValueType(typeID, valueType int) bool {
	t := rules[typeID].valueType
	if t == nil {
		return false
	}

	switch v := t.(type) {
	case int:
		value := v
		if valueType != value {
			return false
		}
	case interval:
		interval := t.(interval)
		if valueType < interval.min || interval.max < valueType {
			return false
		}
	}

	return true
}

func isValidUnit(typeID, unit int) bool {
	u := rules[typeID].unit
	if u == nil {
		return false
	}

	switch v := u.(type) {
	case int:
		value := v
		if unit != value {
			return false
		}
	case interval:
		interval := u.(interval)
		if unit < interval.min || interval.max < unit {
			return false
		}
	}

	return true
}

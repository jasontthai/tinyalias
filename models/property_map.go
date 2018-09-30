package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
)

/*PropertyMap is the type for our properties field */
type PropertyMap map[string]interface{}

/*Value is the interface (driver.Valuer) that transforms our type
  to a database driver compatible type (marshall the map to JSONB)*/
func (p PropertyMap) Value() (driver.Value, error) {
	j, err := json.Marshal(p)
	return j, err
}

/*Scan is the second interface (sql.Scanner), which takes the raw data from the database
  and transforms it to our type (unmarshal the JSONB([]byte) to PropertyMap type)*/
func (p *PropertyMap) Scan(src interface{}) error {
	v := reflect.ValueOf(src)
	if !v.IsValid() || v.IsNil() {
		return nil
	}

	source, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("Type assertion .([]byte) failed.")
	}

	var i interface{}

	err := json.Unmarshal(source, &i)
	if err != nil {
		return err
	}

	if i == nil {
		// There were no properties specified for this record
		return nil
	}

	*p, ok = i.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Type assertion .(map[string]interface{}) failed.")
	}

	return nil
}

// Merge trees represented as PropertyMap from rhs into lhs with keys from rhs taking precedent.
func (lhs PropertyMap) Merge(rhs PropertyMap) {
	for key, _ := range rhs {
		// Key exists in both lhs and rhs.
		if _, ok := lhs[key]; ok {
			var lhsOk, rhsOk bool
			var lhsPropertyMap, rhsPropertyMap PropertyMap
			var lhsMap, rhsMap map[string]interface{}
			lhsPropertyMap, lhsOk = lhs[key].(PropertyMap)
			if !lhsOk {
				lhsMap, lhsOk = lhs[key].(map[string]interface{})
				if lhsOk {
					lhsPropertyMap = PropertyMap(lhsMap)
				}
			}
			rhsPropertyMap, rhsOk = rhs[key].(PropertyMap)
			if !rhsOk {
				rhsMap, rhsOk = rhs[key].(map[string]interface{})
				if rhsOk {
					rhsPropertyMap = PropertyMap(rhsMap)
				}
			}
			// Value in lhs and rhs are PropertyMap, recursively merge these subtrees.
			if lhsOk && rhsOk {
				lhsPropertyMap.Merge(rhsPropertyMap)
			} else {
				// Overwrite the value in the lhs with the value from the rhs.
				lhs[key] = rhs[key]
			}
		} else {
			// Key exists only in rhs, copy into lhs.
			lhs[key] = rhs[key]
		}
	}
}

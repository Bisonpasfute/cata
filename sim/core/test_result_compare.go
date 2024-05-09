package core

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/wowsims/cata/sim/core/proto"
)

func compareValue(t *testing.T, loc string, vst reflect.Value, vmt reflect.Value, baseFloatTolerance float64) {
	switch vst.Kind() {
	case reflect.Pointer, reflect.Interface:
		if vst.IsNil() && vmt.IsNil() {
			break
		}
		if vst.IsNil() != vmt.IsNil() {
			t.Logf("%s: Expected %v but is %v in multi threaded result!", loc, vst.IsNil(), vmt.IsNil())
			t.Fail()
			break
		}
		compareValue(t, loc, vst.Elem(), vmt.Elem(), baseFloatTolerance)
	case reflect.Struct:
		compareStruct(t, loc, vst, vmt, baseFloatTolerance)
	case reflect.Int32, reflect.Int, reflect.Int64:
		if vst.Int() != vmt.Int() {
			t.Logf("%s: Expected %d but is %d for multi threaded result!", loc, vst.Int(), vmt.Int())
			t.Fail()
		}
	case reflect.Float64:
		tolerance := baseFloatTolerance
		if strings.Contains(loc, "CastTimeMs") {
			tolerance = 2.2 // Castime is rounded in results and may be off 1ms per thread. In test=true sims concurrency is set to 3, 2ms diff seems to never be broken then)
		} else if strings.Contains(loc, "SumSq") {
			tolerance *= 100 // Squared sums can be off more, and as an extension also the stdevs
		} else if strings.Contains(loc, "Stdev") {
			tolerance *= 10 // Squared sums can be off more, and as an extension also the stdevs
		} else if strings.Contains(loc, "Resources") {
			tolerance *= 10 // Seems to do some rounding at some point?
		}
		if math.Abs(vst.Float()-vmt.Float()) > tolerance {
			t.Logf("%s: Expected %f but is %f for multi threaded result!", loc, vst.Float(), vmt.Float())
			t.Fail()
		}
	case reflect.String:
		if vst.String() != vmt.String() {
			t.Logf("%s: Expected %s but is %s for multi threaded result!", loc, vst.String(), vmt.String())
			t.Fail()
		}
	case reflect.Bool:
		if vst.Bool() != vmt.Bool() {
			t.Logf("%s: Expected %t but is %t for multi threaded result!", loc, vst.Bool(), vmt.Bool())
			t.Fail()
		}
	case reflect.Slice, reflect.Array:
		if vst.Len() != vmt.Len() {
			t.Logf("%s: Expected length %d but is %d for multi threaded result!", loc, vst.Len(), vmt.Len())
			t.Fail()
			break
		}
		for i := 0; i < vst.Len(); i++ {
			compareValue(t, fmt.Sprintf("%s[%d]", loc, i), vst.Index(i), vmt.Index(i), baseFloatTolerance)
		}
	case reflect.Map:
		if vst.Len() != vmt.Len() {
			t.Logf("%s: Expected length %d but is %d for multi threaded result!", loc, vst.Len(), vmt.Len())
			t.Fail()
			break
		}
		for _, key := range vst.MapKeys() {
			mtVal := vmt.MapIndex(key)
			keyStr := ""
			switch key.Kind() {
			case reflect.Int32, reflect.Int, reflect.Int64:
				keyStr = fmt.Sprintf("%d", key.Int())
			default:
				keyStr = key.String()
			}
			if !mtVal.IsValid() {
				t.Logf("%s: Key %v not found in multi threaded result!", loc, keyStr)
				t.Fail()
				break
			}
			compareValue(t, fmt.Sprintf("%s[%s]", loc, keyStr), vst.MapIndex(key), mtVal, baseFloatTolerance)
		}
	default:
		t.Logf("%s: Has unhandled kind %s!", loc, vst.Kind().String())
		t.Fail()
	}
}

func checkActionMetrics(t *testing.T, loc string, st []*proto.ActionMetrics, mt []*proto.ActionMetrics, baseFloatTolerance float64) {
	actions := map[string]*proto.ActionMetrics{}

	for _, mtAction := range mt {
		_, exists := actions[mtAction.Id.String()]
		if exists {
			t.Logf("%s.Actions: %s exists multiple times in multi threaded results!", loc, mtAction.Id.String())
			t.Fail()
			continue
		}
		actions[mtAction.Id.String()] = mtAction
	}

	for _, stAction := range st {
		mtAction, exists := actions[stAction.Id.String()]
		if !exists {
			t.Logf("%s.Actions: %s does not exist in multi threaded results!", loc, mtAction.Id.String())
			t.Fail()
			continue
		}

		if stAction.IsMelee != mtAction.IsMelee {
			t.Logf("%s.Actions: %s expected IsMelee = %t but was %t in multi threaded results!", loc, stAction.Id.String(), stAction.IsMelee, mtAction.IsMelee)
			t.Fail()
			continue
		}

		compareValue(t, fmt.Sprintf("%s.Actions[%s]", loc, stAction.Id.String()), reflect.ValueOf(stAction.Targets), reflect.ValueOf(mtAction.Targets), baseFloatTolerance)
	}
}

func checkResourceMetrics(t *testing.T, loc string, st []*proto.ResourceMetrics, mt []*proto.ResourceMetrics, baseFloatTolerance float64) {
	resources := map[string]*proto.ResourceMetrics{}

	rkey := func(r *proto.ResourceMetrics) string {
		return fmt.Sprintf("%s %s", r.Id.String(), r.Type.String())
	}

	for _, mtResource := range mt {
		key := rkey(mtResource)
		_, exists := resources[key]
		if exists {
			t.Logf("%s.Resources: %v exists multiple times in multi threaded results!", loc, key)
			t.Fail()
			continue
		}
		resources[key] = mtResource
	}

	for _, stResource := range st {
		stKey := rkey(stResource)
		mtResource, exists := resources[stKey]
		if !exists {
			t.Logf("%s.Resources: %s does not exist in multi threaded results!", loc, stKey)
			t.Fail()
			continue
		}

		compareValue(t, fmt.Sprintf("%s.Resources[%s]", loc, stKey), reflect.ValueOf(stResource), reflect.ValueOf(mtResource), baseFloatTolerance)
	}
}

func compareStruct(t *testing.T, loc string, vst reflect.Value, vmt reflect.Value, baseFloatTolerance float64) {
	for i := 0; i < vst.NumField(); i++ {
		fieldName := vst.Type().Field(i).Name
		fieldType := vst.Type().Field(i).Type.Name()

		if fieldType == "MessageState" {
			continue
		}

		stField := vst.Field(i)
		mtField := vmt.Field(i)

		if stField.Kind() == reflect.Ptr {
			if stField.IsNil() && mtField.IsNil() {
				continue
			} else if stField.IsNil() != mtField.IsNil() {
				t.Logf("%s.%s: Expected %v but is %v in multi threaded result!", loc, fieldName, stField.IsNil(), mtField.IsNil())
				t.Fail()
				continue
			}

			stField = stField.Elem()
			mtField = mtField.Elem()
		}

		if fieldName == "Actions" {
			checkActionMetrics(t, loc, stField.Interface().([]*proto.ActionMetrics), mtField.Interface().([]*proto.ActionMetrics), baseFloatTolerance)
			continue
		} else if fieldName == "Resources" {
			checkResourceMetrics(t, loc, stField.Interface().([]*proto.ResourceMetrics), mtField.Interface().([]*proto.ResourceMetrics), baseFloatTolerance)
			continue
		}

		compareValue(t, fmt.Sprintf("%s.%s", loc, fieldName), stField, mtField, baseFloatTolerance)
	}
}

func CompareConcurrentSimResultsTest(t *testing.T, testName string, singleThreadRes *proto.RaidSimResult, multiThreadRes *proto.RaidSimResult, baseFloatTolerance float64) {
	t.Run(testName+"/CompareResults", func(t *testing.T) {
		vst := reflect.ValueOf(singleThreadRes).Elem()
		vmt := reflect.ValueOf(multiThreadRes).Elem()
		compareStruct(t, "RaidSimResult", vst, vmt, baseFloatTolerance)
		if t.Failed() {
			t.Log("A fail here means that either the combination of results is broken, or there's a state leak between iterations!")
		}
	})
}

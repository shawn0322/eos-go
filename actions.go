package eos

import (
	"encoding/json"
	"reflect"
)

var registeredActions = map[AccountName]map[ActionName]reflect.Type{}

// Registers Action objects..
func RegisterAction(accountName AccountName, actionName ActionName, obj interface{}) {
	// TODO: lock or som'th.. unless we never call after boot time..
	if registeredActions[accountName] == nil {
		registeredActions[accountName] = make(map[ActionName]reflect.Type)
	}
	registeredActions[accountName][actionName] = reflect.ValueOf(obj).Type()
}

// See: libraries/chain/include/eosio/chain/contracts/types.hpp:203
// See: build/contracts/eosio.system/eosio.system.abi

// SetCode represents the hard-coded `setcode` action.
type SetCode struct {
	Account   AccountName `json:"account"`
	VMType    byte        `json:"vmtype"`
	VMVersion byte        `json:"vmversion"`
	Code      HexBytes    `json:"bytes"`
}

// SetABI represents the hard-coded `setabi` action.
type SetABI struct {
	Account AccountName `json:"account"`
	ABI     ABI         `json:"abi"`
}

// NewAccount represents the hard-coded `newaccount` action.
type NewAccount struct {
	Creator  AccountName `json:"creator"`
	Name     AccountName `json:"name"`
	Owner    Authority   `json:"owner"`
	Active   Authority   `json:"active"`
	Recovery Authority   `json:"recovery"`
}

// Action
type Action struct {
	Account       AccountName       `json:"account"`
	Name          ActionName        `json:"name"`
	Authorization []PermissionLevel `json:"authorization,omitempty"`
	Data          ActionData        `json:"data"` // as HEX when we receive it.. FIXME: decode from hex directly.. and encode back plz!
}

func (a Action) Obj() interface{} { // Payload ? ActionData ? GetData ?
	return a.Data.obj
}

type ActionData struct {
	HexBytes
	obj interface{} // potentially unpacked from the Actions registry mapped through `RegisterAction`.
	abi []byte      // TBD: we could use the ABI to decode in obj
}

func NewActionData(obj interface{}) ActionData {
	return ActionData{
		HexBytes: []byte(""),
		obj: obj,
	}
}

func (a ActionData) MarshalBinary() ([]byte, error) {
	if a.obj != nil {
		raw, err := MarshalBinary(a.obj)
		if err != nil {
			return nil, err
		}
		a.HexBytes = HexBytes(raw)
	}
	return MarshalBinary(a.HexBytes)
}

func (a *ActionData) UnmarshalBinaryWithLastAction(data []byte, act Action) error {
	a.HexBytes = HexBytes(data)

	actionMap := registeredActions[act.Account]

	var decodeInto reflect.Type
	if actionMap != nil {
		objType := actionMap[act.Name]
		if objType != nil {
			decodeInto = reflect.TypeOf(objType)
		}
	}
	if decodeInto == nil {
		return nil
	}

	obj := reflect.New(decodeInto)

	err := UnmarshalBinary(data, &obj)
	if err != nil {
		return err
	}

	a.obj = obj.Elem().Interface()

	return nil
}

func (a *ActionData) UnmarshalJSON(v []byte) (err error) {
	// Unmarshal from the JSON format ?  We'd need it to be registered.. but we can't hook into the JSON
	// lib to read the current action above.. we'll need to defer loading
	// Either keep as json.RawMessage, or as map[string]interface{}
	a.HexBytes = v
	return nil
}

// DecodeAs allows you to decode after the fact, either from JSON or from HexBytes
func (a *Action) DecodeAs(v interface{}) (err error) {
	if msg, ok := a.Data.obj.(json.RawMessage); ok {
		err = json.Unmarshal(msg, v)
	} else {
		err = UnmarshalBinary(a.Data.HexBytes, v)
	} // Fail with an error if HexBytes was len=0 ?
	// Perhaps it was already decoded into something Registered!
	if err != nil {
		return err
	}

	a.Data.obj = v

	return nil

}

func (a ActionData) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.obj)
}

type jsonAction struct {
	Account       AccountName       `json:"account"`
	Name          ActionName        `json:"name"`
	Authorization []PermissionLevel `json:"authorization,omitempty"`
	Data          HexBytes          `json:"data"`
}

func (a *Action) UnmarshalJSON(v []byte) (err error) {
	// load Account, Name, Authorization, Data
	// and then unpack other fields in a struct based on `Name` and `AccountName`..
	var newAct jsonAction
	if err = json.Unmarshal(v, &newAct); err != nil {
		return
	}

	a.Account = newAct.Account
	a.Name = newAct.Name
	a.Authorization = newAct.Authorization
	a.Data.HexBytes = newAct.Data

	// err = UnmarshalBinaryWithAction([]byte(newAct.Data), &a.Data, *a)
	// if err != nil {
	// 	return err
	// }

	return nil
}

func (a *Action) MarshalJSON() ([]byte, error) {
	var data HexBytes
	if a.Data.obj == nil {
		data = a.Data.HexBytes
	} else {
		var err error
		data, err = MarshalBinary(a.Data.obj)
		if err != nil {
			return nil, err
		}
	}

	return json.Marshal(&jsonAction{
		Account:       a.Account,
		Name:          a.Name,
		Authorization: a.Authorization,
		Data:          HexBytes(data),
	})
}

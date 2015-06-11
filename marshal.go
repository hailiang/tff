package teff

import (
	"bytes"
	"fmt"
	"h12.me/teff/core"
	"io"
	"reflect"
	"strconv"
)

func Marshal(v interface{}) ([]byte, error) {
	return MarshalIndent(v, "", "\t")
}

func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	var w bytes.Buffer
	err := NewEncoder(&w).marshalIndent(v, prefix, indent)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func Unmarshal(data []byte, v interface{}) error {
	if string(data) == "nil" {
		return nil
	}
	list, err := core.Parse(bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	return unmarshalList(list, reflect.ValueOf(v))
}

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (enc *Encoder) Encode(v interface{}) error {
	return nil
}

func (enc *Encoder) marshalIndent(v interface{}, prefix, indent string) error {
	var list core.List
	var err error
	if v == nil {
		list = core.List{core.Node{Value: "nil"}}
	} else {
		list, err = marshalList(reflect.ValueOf(v))
		if err != nil {
			return err
		}
	}
	return list.Marshal(enc.w, prefix, indent)
}

func marshalList(v reflect.Value) (core.List, error) {
	switch v.Type().Kind() {
	case reflect.Int, reflect.String:
		node, err := marshalNode(v)
		if err != nil {
			return nil, err
		}
		return core.List{node}, nil
	case reflect.Slice:
		list := make(core.List, v.Len())
		for i := 0; i < v.Len(); i++ {
			node, err := marshalNode(v.Index(i))
			if err != nil {
				return nil, err
			}
			list[i] = node
		}
		return list, nil
	case reflect.Ptr:
		return marshalList(reflectValue(v))
	}
	return nil, fmt.Errorf("marshal unsupported")
}

func unmarshalList(list core.List, v reflect.Value) error {
	switch v.Type().Kind() {
	case reflect.Int, reflect.String:
		return unmarshalNode(list[0], v)
	case reflect.Slice:
		for i, node := range list {
			v.Set(reflect.Append(v, reflect.New(v.Type().Elem()).Elem()))
			elem := v.Index(i)
			if err := unmarshalNode(node, elem); err != nil {
				return err
			}
		}
		return nil
	case reflect.Ptr:
		return unmarshalList(list, allocValue(v))
	}
	return fmt.Errorf("unmarshal unsupported")
}

func marshalNode(v reflect.Value) (core.Node, error) {
	switch v.Type().Kind() {
	case reflect.Int:
		return core.Node{Value: fmt.Sprint(v.Interface())}, nil
	case reflect.String:
		s := v.Interface().(string)
		if !strconv.CanBackquote(s) {
			s = strconv.Quote(s)
		}
		return core.Node{Value: s}, nil
	case reflect.Ptr:
		return marshalNode(v.Elem())
	}
	return core.Node{}, fmt.Errorf("marshal unsupported")

}

func unmarshalNode(node core.Node, v reflect.Value) error {
	switch v.Type().Kind() {
	case reflect.Int:
		i, err := strconv.Atoi(node.Value)
		if err != nil {
			return err
		}
		v.SetInt(int64(i))
		return nil
	case reflect.String:
		s, err := strconv.Unquote(node.Value)
		if err != nil {
			s = node.Value
		}
		v.SetString(s)
		return nil
	case reflect.Ptr:
		return unmarshalNode(node, allocValue(v))
	}
	return fmt.Errorf("unmarshal unsupported")
}

func reflectValue(v reflect.Value) reflect.Value {
	for v.Type().Kind() == reflect.Ptr && !v.IsNil() {
		v = reflect.Indirect(v)
	}
	return v
}

func allocValue(v reflect.Value) reflect.Value {
	for v.Type().Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = reflect.Indirect(v)
	}
	return v
}

/*
Package model converts almost any Go data structures into a tree model:
1. cyclic references are replaced with labels
2. pointer & interfaces are replaced with values
3. reflections are hidden from outside if possible
*/
package model

import (
	"errors"
	"reflect"
	"strconv"
)

type (
	List []*Node
	Node struct {
		Label Label
		Value interface{}
		List  List
	}
	Label      string
	IdentValue struct {
		Ident Identifier
		Value interface{}
	}
	Identifier string
)

func New(v interface{}) (List, error) {
	if v == nil {
		return nil, nil
	}
	return newMaker().list(reflect.ValueOf(v))
}

func Fill(l List, v interface{}) error {
	if v == nil {
		return nil
	}
	return newFiller().fromList(l, reflect.ValueOf(v))
}

func (m *maker) list(v reflect.Value) (List, error) {
	switch v.Type().Kind() {
	case reflect.Int, reflect.String:
		node, err := m.node(v)
		if err != nil {
			return nil, err
		}
		return List{node}, nil
	case reflect.Slice:
		return m.listFromSlice(v)
	case reflect.Struct:
		return m.listFromStruct(v)
	case reflect.Ptr:
		return m.list(indirect(v))
	}
	return nil, errors.New("maker.list: unsupported type")
}

func (f *filler) fromList(l List, v reflect.Value) error {
	switch v.Type().Kind() {
	case reflect.Int, reflect.String:
		if len(l) > 0 {
			return f.fromNode(l[0], v)
		}
	case reflect.Slice:
		return f.sliceFromList(l, v)
	case reflect.Struct:
		return f.structFromList(l, v)
	case reflect.Ptr:
		return f.fromList(l, allocIndirect(v))
	}
	return errors.New("List.fill: unsupported type")
}

func (m *maker) listFromSlice(v reflect.Value) (List, error) {
	l := make(List, v.Len())
	for i := 0; i < v.Len(); i++ {
		node, err := m.node(v.Index(i))
		if err != nil {
			return nil, err
		}
		l[i] = node
	}
	return l, nil
}

func (f *filler) sliceFromList(l List, v reflect.Value) error {
	for i, n := range l {
		v.Set(reflect.Append(v, reflect.New(v.Type().Elem()).Elem()))
		elem := v.Index(i)
		if err := f.fromNode(n, elem); err != nil {
			return err
		}
	}
	return nil
}

func (m *maker) listFromStruct(v reflect.Value) (List, error) {
	l := make(List, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		node, err := m.node(v.Field(i))
		if err != nil {
			return nil, err
		}
		if node.List == nil {
			node.Value = IdentValue{Identifier(v.Type().Field(i).Name), node.Value}
		}
		l[i] = node
	}
	return l, nil
}

func (f *filler) structFromList(l List, v reflect.Value) error {
	for _, n := range l {
		if iv, ok := n.Value.(IdentValue); ok {
			if field := v.FieldByName(string(iv.Ident)); field.IsValid() {
				if err := f.fromNode(n, field); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *maker) node(v reflect.Value) (*Node, error) {
	switch v.Type().Kind() {
	case reflect.Int:
		return &Node{Value: int(v.Int())}, nil
	case reflect.String:
		return &Node{Value: v.String()}, nil
	case reflect.Slice:
		list, err := m.list(v)
		if err != nil {
			return nil, err
		}
		return &Node{List: list}, nil
	case reflect.Ptr:
		return m.nodeFromPtr(v)
	}
	return nil, errors.New("node: unsupported type")
}

func (f *filler) fromNode(n *Node, v reflect.Value) error {
	switch v.Type().Kind() {
	case reflect.Int, reflect.String:
		switch src := n.Value.(type) {
		case IdentValue:
			if label, ok := src.Value.(Label); ok {
				p(label)
			} else {
				v.Set(reflect.ValueOf(src.Value))
			}
		default:
			v.Set(reflect.ValueOf(src))
		}
		return nil
	case reflect.Slice:
		return f.fromList(n.List, v)
	case reflect.Ptr:
		return f.ptrFromNode(n, v)
	}
	return errors.New("Node.fill: unsupported type")
}

func (m *maker) nodeFromPtr(v reflect.Value) (*Node, error) {
	if v.IsNil() {
		return &Node{}, nil // avoid infinite loop
	}
	addr := v.Pointer()
	if label, ok := m.label(addr); ok {
		return &Node{Value: label}, nil
	}
	node, err := m.node(indirect(v))
	if err != nil {
		return nil, err
	}
	m.register(addr, node)
	return node, nil
}

func (f *filler) ptrFromNode(n *Node, v reflect.Value) error {
	if n.Label != "" {
		f.register(n.Label, v)
	} else if label, ok := n.Value.(Label); ok {
		v.Set(f.value(label))
		return nil
	}
	return f.fromNode(n, allocIndirect(v))
}

// maker makes a new List
type maker struct {
	m      map[uintptr]*Node
	serial int
}

// filler fills from a list
type filler struct {
	m map[Label]reflect.Value
}

func newFiller() *filler {
	return &filler{make(map[Label]reflect.Value)}
}

func newMaker() *maker {
	return &maker{
		m:      make(map[uintptr]*Node),
		serial: 1,
	}
}

func (m *maker) register(p uintptr, node *Node) {
	m.m[p] = node
}

func (f *filler) register(label Label, v reflect.Value) {
	f.m[label] = v
}

func (m *maker) label(addr uintptr) (Label, bool) {
	if node, ok := m.m[addr]; ok {
		if node.Label == "" {
			node.Label = Label(strconv.Itoa(m.serial))
			m.serial++
		}
		return node.Label, true
	}
	return Label(0), false
}

func (f *filler) value(label Label) reflect.Value {
	return f.m[label]
}

func indirect(v reflect.Value) reflect.Value {
	for v.Type().Kind() == reflect.Ptr && !v.IsNil() {
		v = reflect.Indirect(v)
	}
	return v
}

func allocIndirect(v reflect.Value) reflect.Value {
	for v.Type().Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = reflect.Indirect(v)
	}
	return v
}

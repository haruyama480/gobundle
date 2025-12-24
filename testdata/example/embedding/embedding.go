package embedding

import (
	"example.com/mypackage/embedding/child"
)

type parent struct {
	*child.Child
	Hoge int
}

type parent2 struct {
	*parent
}

type parent3 struct {
	parent2
	child.Child
}

func (p parent) Greet() {
	p.Child.Greet()
}

func Exec() {
	p3 := parent3{
		parent2: parent2{
			parent: &parent{
				Child: &child.Child{Name: "World"},
				Hoge:  42,
			},
		},
	}
	p3.parent2.parent.Child.Greet()
	p3.parent2.Child.Greet()
	p3.parent.Child.Greet()
	p3.Child.Greet()
}

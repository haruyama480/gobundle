package parent

import (
	"example.com/mypackage/child"
)

var ParentVar = child.ChildVar

func ParentFunc() string {
	x := ParentVar
	return x
}

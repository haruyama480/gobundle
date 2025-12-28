# gobundle

installation
```
go install github.com/haruyama480/gobundle/cmd/gobundle
```

example
```
git clone https://github.com/haruyama480/gobundle/
cd gobundle/testdata/example
gobundle main.go
```

gobundle doesn't support
- `//go:embed`
- initialization order
- and following TODOs

TODO
- dot import
  - > If an explicit period (.) appears instead of a name, all the package's exported identifiers declared in that package's package block will be declared in the importing source file's file block and must be accessed without a qualifier.
  - https://go.dev/ref/spec#Import_declarations
  - ネストしたdot importの可視性は伝播しないことに注意
- underscore import
- オプション化
  - dstPkgName
  - retainFn
  - コメント狩り
  - deadcode狩り
- コメントサポートの範囲
  - posを保持してないのもあるのでいくつか消える可能性がありそう
- testtesttest
- GetPackageNameFromPathの代替

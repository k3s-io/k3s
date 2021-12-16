# v0.7.10 - 2021/10/16

* Fix conversion from pointer to uint64  ( #294 )

# v0.7.9 - 2021/09/28

* Fix encoding of nil value about interface type that has method ( #291 )

# v0.7.8 - 2021/09/01

* Fix mapassign_faststr for indirect struct type ( #283 )
* Fix encoding of not empty interface type ( #284 )
* Fix encoding of empty struct interface type ( #286 )

# v0.7.7 - 2021/08/25

* Fix invalid utf8 on stream decoder ( #279 )
* Fix buffer length bug on string stream decoder ( #280 )

Thank you @orisano !!

# v0.7.6 - 2021/08/13

* Fix nil slice assignment ( #276 )
* Improve error message ( #277 )

# v0.7.5 - 2021/08/12

* Fix encoding of embedded struct with tags ( #265 )
* Fix encoding of embedded struct that isn't first field ( #272 )
* Fix decoding of binary type with escaped char ( #273 )

# v0.7.4 - 2021/07/06

* Fix encoding of indirect layout structure ( #264 )

# v0.7.3 - 2021/06/29

* Fix encoding of pointer type in empty interface ( #262 )

# v0.7.2 - 2021/06/26

### Fix decoder

* Add decoder for func type to fix decoding of nil function value ( #257 )
* Fix stream decoding of []byte type ( #258 )

### Performance

* Improve decoding performance of map[string]interface{} type ( use `mapassign_faststr` ) ( #256 )
* Improve encoding performance of empty interface type ( remove recursive calling of `vm.Run` ) ( #259 )

### Benchmark

* Add bytedance/sonic as benchmark target ( #254 )

# v0.7.1 - 2021/06/18

### Fix decoder

* Fix error when unmarshal empty array ( #253 )

# v0.7.0 - 2021/06/12

### Support context for MarshalJSON and UnmarshalJSON ( #248 )

* json.MarshalContext(context.Context, interface{}, ...json.EncodeOption) ([]byte, error)
* json.NewEncoder(io.Writer).EncodeContext(context.Context, interface{}, ...json.EncodeOption) error
* json.UnmarshalContext(context.Context, []byte, interface{}, ...json.DecodeOption) error
* json.NewDecoder(io.Reader).DecodeContext(context.Context, interface{}) error

```go
type MarshalerContext interface {
  MarshalJSON(context.Context) ([]byte, error)
}

type UnmarshalerContext interface {
  UnmarshalJSON(context.Context, []byte) error
}
```

### Add DecodeFieldPriorityFirstWin option ( #242 )

In the default behavior, go-json, like encoding/json, will reflect the result of the last evaluation when a field with the same name exists. I've added new options to allow you to change this behavior. `json.DecodeFieldPriorityFirstWin` option reflects the result of the first evaluation if a field with the same name exists. This behavior has a performance advantage as it allows the subsequent strings to be skipped if all fields have been evaluated.

### Fix encoder

* Fix indent number contains recursive type ( #249 )
* Fix encoding of using empty interface as map key ( #244 )

### Fix decoder

* Fix decoding fields containing escaped characters ( #237 )

### Refactor

* Move some tests to subdirectory ( #243 )
* Refactor package layout for decoder ( #238 )

# v0.6.1 - 2021/06/02

### Fix encoder

* Fix value of totalLength for encoding ( #236 )

# v0.6.0 - 2021/06/01

### Support Colorize option for encoding (#233)

```go
b, err := json.MarshalWithOption(v, json.Colorize(json.DefaultColorScheme))
if err != nil {
  ...
}
fmt.Println(string(b)) // print colored json
```

### Refactor

* Fix opcode layout - Adjust memory layout of the opcode to 128 bytes in a 64-bit environment ( #230 )
* Refactor encode option ( #231 )
* Refactor escape string ( #232 )

# v0.5.1 - 2021/5/20

### Optimization

* Add type addrShift to enable bigger encoder/decoder cache ( #213 )

### Fix decoder

* Keep original reference of slice element ( #229 )

### Refactor

* Refactor Debug mode for encoding ( #226 )
* Generate VM sources for encoding ( #227 )
* Refactor validator for null/true/false for decoding ( #221 )

# v0.5.0 - 2021/5/9

### Supports using omitempty and string tags at the same time ( #216 )

### Fix decoder

* Fix stream decoder for unicode char ( #215 )
* Fix decoding of slice element ( #219 )
* Fix calculating of buffer length for stream decoder ( #220 )

### Refactor

* replace skipWhiteSpace goto by loop ( #212 )

# v0.4.14 - 2021/5/4

### Benchmark

* Add valyala/fastjson to benchmark ( #193 )
* Add benchmark task for CI ( #211 )

### Fix decoder

* Fix decoding of slice with unmarshal json type ( #198 )
* Fix decoding of null value for interface type that does not implement Unmarshaler ( #205 )
* Fix decoding of null value to []byte by json.Unmarshal ( #206 )
* Fix decoding of backslash char at the end of string ( #207 )
* Fix stream decoder for null/true/false value ( #208 )
* Fix stream decoder for slow reader ( #211 )

### Performance

* If cap of slice is enough, reuse slice data for compatibility with encoding/json ( #200 )

# v0.4.13 - 2021/4/20

### Fix json.Compact and json.Indent

* Support validation the input buffer for json.Compact and json.Indent ( #189 )
* Optimize json.Compact and json.Indent ( improve memory footprint ) ( #190 )

# v0.4.12 - 2021/4/15

### Fix encoder

* Fix unnecessary indent for empty slice type ( #181 )
* Fix encoding of omitempty feature for the slice or interface type ( #183 )
* Fix encoding custom types zero values with omitempty when marshaller exists ( #187 )

### Fix decoder

* Fix decoder for invalid top level value ( #184 )
* Fix decoder for invalid number value ( #185 )

# v0.4.11 - 2021/4/3

* Improve decoder performance for interface type

# v0.4.10 - 2021/4/2

### Fix encoder

* Fixed a bug when encoding slice and map containing recursive structures
* Fixed a logic to determine if indirect reference

# v0.4.9 - 2021/3/29

### Add debug mode

If you use `json.MarshalWithOption(v, json.Debug())` and `panic` occurred in `go-json`, produces debug information to console.

### Support a new feature to compatible with encoding/json

- invalid UTF-8 is coerced to valid UTF-8 ( without performance down )

### Fix encoder

- Fixed handling of MarshalJSON of function type

### Fix decoding of slice of pointer type

If there is a pointer value, go-json will use it. (This behavior is necessary to achieve the ability to prioritize pre-filled values). However, since slices are reused internally, there was a bug that referred to the previous pointer value. Therefore, it is not necessary to refer to the pointer value in advance for the slice element, so we explicitly initialize slice element by `nil`.

# v0.4.8 - 2021/3/21

### Reduce memory usage at compile time

* go-json have used about 2GB of memory at compile time, but now it can compile with about less than 550MB.

### Fix any encoder's bug

* Add many test cases for encoder
* Fix composite type ( slice/array/map )
* Fix pointer types
* Fix encoding of MarshalJSON or MarshalText or json.Number type

### Refactor encoder

* Change package layout for reducing memory usage at compile
* Remove anonymous and only operation
* Remove root property from encodeCompileContext and opcode

### Fix CI

* Add Go 1.16
* Remove Go 1.13
* Fix `make cover` task

### Number/Delim/Token/RawMessage use the types defined in encoding/json by type alias

# v0.4.7 - 2021/02/22

### Fix decoder

* Fix decoding of deep recursive structure
* Fix decoding of embedded unexported pointer field
* Fix invalid test case
* Fix decoding of invalid value
* Fix decoding of prefilled value
* Fix not being able to return UnmarshalTypeError when it should be returned
* Fix decoding of null value
* Fix decoding of type of null string
* Use pre allocated pointer if exists it at decoding

### Reduce memory usage at compile

* Integrate int/int8/int16/int32/int64 and uint/uint8/uint16/uint32/uint64 operation to reduce memory usage at compile

### Remove unnecessary optype

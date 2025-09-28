# golem

<img align="right" width="180px" src="https://raw.githubusercontent.com/leandroluk/golem/refs/heads/master/assets/golem.png">

[![Build Status](https://github.com/leandroluk/golem/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/leandroluk/golem/actions)  
[![Coverage Status](https://img.shields.io/codecov/c/github/leandroluk/golem/main.svg)](https://codecov.io/gh/leandroluk/golem)  
[![Go Report Card](https://goreportcard.com/badge/github.com/leandroluk/golem)](https://goreportcard.com/report/github.com/leandroluk/golem)  
[![Go Doc](https://godoc.org/github.com/leandroluk/golem?status.svg)](https://pkg.go.dev/github.com/leandroluk/golem)  
[![Release](https://img.shields.io/github/release/leandroluk/golem.svg?style=flat-square)](https://github.com/leandroluk/golem/releases)  

A type-safe ORM for Go, built to be simple and strong.

---

## Contents
- [golem](#golem)
  - [Contents](#contents)
  - [Getting started](#getting-started)
  - [Usage](#usage)
  - [Available Schemas](#available-schemas)
  - [Examples](#examples)
    - [String Validation](#string-validation)
    - [Number Validation](#number-validation)
    - [Boolean Validation](#boolean-validation)
    - [Object Validation](#object-validation)
  - [Implementation Status](#implementation-status)
  - [About the Project](#about-the-project)
  - [Contributors](#contributors)
  - [License](#license)

---

## Getting started

1. Install the package:

```sh
go get github.com/leandroluk/golem
```

2. Import it in your project:

```go
import "github.com/leandroluk/golem"
```

3. Create and validate schemas:

```go
schema := joi.Object(map[string]joi.Schema{
    "username": joi.String().Min(3).Max(20).Required(),
    "age":      joi.Number().Min(18).Required(),
    "email":    joi.String().Regex(regexp.MustCompile(`.+@.+\..+`)),
})

value := map[string]any{
    "username": "john_doe",
    "age":      25,
    "email":    "john@example.com",
}

parsed, errs := schema.Validate("user", value)
if len(errs) > 0 {
    fmt.Println("Validation failed:", errs)
} else {
    fmt.Println("Validation passed:", parsed)
}
```

---

## Usage

- **Basic types**: `String`, `Number`, `Boolean`, `Object`
- **Rules**:  
  - String: `.Min()`, `.Max()`, `.Regex()`, `.Trim()`, `.Lowercase()`, `.Uppercase()`  
  - Number: `.Min()`, `.Max()`, `.Integer()`, `.Positive()`, `.Negative()`  
  - Boolean: `.True()`, `.False()`, `.Truthy()`, `.Falsy()`  
  - Object: `.Min()`, `.Max()`, `.Length()`, `.Unknown()`  

---

## Available Schemas
- ✅ String
- ✅ Number
- ✅ Boolean
- ✅ Object
- ⬜ Array *(coming soon)*

---

## Examples

### String Validation
```go
joi.String().Min(5).Max(10).Trim()
```

### Number Validation
```go
joi.Number().Integer().Positive()
```

### Boolean Validation
```go
joi.Boolean().Truthy("yes", "1").Falsy("no", "0")
```

### Object Validation
```go
joi.Object(map[string]joi.Schema{
    "id":   joi.Number().Required(),
    "name": joi.String().Min(3),
}).Unknown(false)
```

---

## Implementation Status
- [x] String rules
- [x] Number rules
- [x] Boolean rules
- [x] Object rules
- [ ] Array rules
- [ ] Custom extensions

---

## About the Project
This project was inspired by [joi](https://github.com/hapijs/joi) for Node.js and aims to bring a similar developer experience to Go.

---

## Contributors
Thanks to all the people who contribute! [[Contribute](CONTRIBUTING.md)]

---

## License
MIT License – see [LICENSE](LICENSE) file for details.

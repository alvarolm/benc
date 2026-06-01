# bencgen

A code generator for the **Benc** schema format, ensuring forward and backward compatibility of generated code.

## Table Of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
  - [Example Usage](#example-usage)
- [Generating Example](#generating-example-specifically-go-because-of-go_package)
  - [Go Usage Example](#go-usage-example)
- [Breaking Changes Detector (BCD)](#breaking-changes-detector-bcd)
- [Maintaining](#maintaining)
- [Examples and Tests](#examples-and-tests)
- [Importing Other Benc Files](#importing-other-benc-files)
- [Enums](#enums)
- [Schema Grammar](#schema-grammar)
  - [Comments](#comments)
  - [Define](#define)
  - [Fields](#fields)
  - [Type Attributes](#type-attributes)
  - [Types](#types)
  - [Containers or Enums](#containers-or-enums)
  - [Fixed-Size Arrays](#fixed-size-arrays)
  - [Custom Types](#custom-types)
- [Languages](#languages)
- [License](#license)

## Requirements

- **Go** (for installing and running `bencgen`)
- [Benc Standard](../../std/README.md)
- [Bencgen Implementation](../../impl/gen/README.md)

## Installation

1. Install `bencgen` using the following command:

   ```bash
   go install github.com/alvarolm/benc/cmd/bencgen
   ```

## Usage

### Arguments

- `--in`: Comma-separated list of input `.benc` files (required, no spaces allowed)
- `--out`: Output directory (optional; use `...` to replace with input filename)
- `--lang`: Target [language](#languages) to compile into (required)
- `--file`: Name of the output file (optional; use `...` to replace with input filename)
- `--force`: Disable the breaking changes detector (optional, not recommended in production)
- `--import-dir`: Comma-separated list of directories to import files from (optional, no spaces allowed)

Find a complex bencgen usage example [here](#importing-other-benc-files).

### Example Usage

```bash
bencgen --in person.benc --lang go --out ./output --file ..._result
```

This command will process the `person.benc` file, generate Go code, and save the result as `person_result.go` in the `output` directory.

## Generating Example (specifically go, because of go_package)

1. Create a `.benc` file (e.g., `person.benc`).
2. Write a schema, for example:

   ```plaintext
   define person;

   var go_package = "github.com/.../output/person";

   ctr Person {
       int age = 1;
       string name = 2;
       Parents parents = 3;
       Child child = 4;
   }

   ctr Child {
       int age = 1;
       string name = 2;
       Parents parents = 3;
   }

    ctr Parents {
        string mother = 1;
        string father = 2;
    }
   ```

3. Generate Go code with the following command:

   ```bash
   bencgen --in person.benc --lang go --out ./output/person --file ...
   ```

4. Follow the instructions for using the generated code in the selected language:
   - [Go Usage Example](#go-usage-example)

### Go Usage Example

After generating the code, a file called `output/person/person.go` will be created. Here's how to marshal and unmarshal the `Person` data:

```go
package main

import (
	"github.com/alvarolm/benc"
	"github.com/.../output/person"
)

func main() {
	data := person.Person{
		Age:  24,
		Name: "Johnny",
		Parents: person.Parents{
			Mother: "Johna",
			Father: "John",
		},
		Child: person.Child{
			Name: "Johnny Jr.",
			Age:  3,
			Parents: person.Parents{
				Mother: "Johna Jr.",
				Father: "Johnny",
			},
		},
	}

	buf := make([]byte, data.Size())
	data.Marshal(buf)

	var retData person.Person
	if err := retData.Unmarshal(buf); err != nil {
		panic(err)
	}
}
```

## Breaking Changes Detector (BCD)

BCD helps identify breaking changes, such as:

- A field exists but is marked as reserved.
- A field was removed but isn't marked as reserved.
- The type of a field changed, but its ID remains the same.

## Maintaining Your Schema

To ensure forward and backward compatibility in your Benc schema, follow these best practices:

- **Mark removed fields as reserved.**
- **Append new fields at the bottom** (order fields by their IDs in ascending order).
- If a field's type changes, **assign it a new ID** and mark the old ID as reserved.
- Use the **[BCD](#breaking-changes-detector-bcd)** (enabled by default) to catch and report compatibility issues.

### Reserving IDs Example

If the `parents` field (ID `3`) is removed, you should mark the ID as reserved:

```plaintext
define person;

var go_package = "github.com/.../output/person";

ctr Person {
    reserved 3;  # Reserved the 'parents' field ID

    int age = 1;
    string name = 2;
    Child child = 4;
}

ctr Child {
    int age = 1;
    string name = 2;
    Parents parents = 3;
}

ctr Parents {
    string mother = 1;
    string father = 2;
}
```

## Examples and Tests

See all tests [here](../../testing). For tests related to forward and backward compatibility, check [here](../../testing/person/main_person_test.go).

## Importing Other Benc Files

To reuse Benc schemas across multiple files, you can **import** other schemas using the `use` keyword.

### Example: Importing Files

Assume you have the following structure:

```
/schemas
    /baby.benc
    /babysitter.benc
    /parent.benc
```

**baby.benc:**

```plaintext
define baby.v1;

var go_package = "github.com/.../output/baby";

ctr Baby {
    int age = 1;
    string name = 2;
}
```

**babysitter.benc:**

```plaintext
define babysitter;

use "baby.benc";

var go_package = "github.com/.../output/babysitter";

ctr Babysitter {
    int age = 1;
    string name = 2;
    baby.v1.Baby babyAssignedTo = 3;
}
```

**parent.benc:**

```plaintext
define parent;

use "baby.benc";

var go_package = "github.com/.../output/parent";

ctr Parent {
    int age = 1;
    string name = 2;
    baby.v1.Baby baby = 3;
}
```

To generate the code for all three files:

```bash
bencgen --in schemas/baby.benc,schemas/babysitter.benc,schemas/parent.benc --out output/... --file ... --lang go --import-dir schemas
```

Now, both `babysitter` and `parent` share the same `baby` data.

## Enums

Enums are treated as named integers. Forward and backward compatibility is preserved, even when fields are added or removed from an enum.

### Enum Example:

```plaintext
define person;

var go_package = "github.com/.../output/person";

enum JobStatus {
    Employed,
    Unemployed
}

ctr Person {
    int age = 1;
    string name = 2;
    JobStatus jobStatus = 3;
}
```

## Schema Grammar

A schema consists of the following components:

### Comments

Line comments start with either `#` or `//` and run to the end of the line:

```plaintext
# this is a comment
// so is this
```

A comment attached to a container, field, enum, or enum value is carried through
to the generated code as a doc comment on the corresponding declaration. A
comment is attached when it sits **directly above** the declaration (leading) or
**at the end of the same line** (trailing). When both are present, the leading
comment wins.

```plaintext
// A person record.
ctr Person {
    // The person's age in years.
    int age = 1;
    string name = 2; // the full display name
}

// Supported colors.
enum Color {
    RED,   // the default
    GREEN,
    BLUE,
}
```

generates (Go):

```go
// A person record.
// Struct - Person
type Person struct {
    // The person's age in years.
    Age int
    // the full display name
    Name string
}

// Supported colors.
// Enum - Color
type Color int
const (
    // the default
    ColorRED Color = iota
    ColorGREEN
    ColorBLUE
)
```

Comments are ignored by the [BCD](#breaking-changes-detector-bcd): editing them
never triggers a breaking-change error.

### Define

The `define` statement is used when importing other Benc files to access their enums/containers:

```plaintext
define <IDENTIFIER>;
```

### Fields

A field consists of:

```plaintext
[ATTR] [TYPE] IDENTIFIER = ID;
```

or

```plaintext
[CONTAINER_OR_ENUM_NAME] IDENTIFIER = ID;
```

- **ID**: Must be no larger than `65535`.
- **Type attributes** (`unsafe`, `rcopy`) precede the type.

Example of a simple field:

```plaintext
string name = 1;
bytes data = 2;
```

Example of a field with type attributes:

```plaintext
unsafe string name = 1;
rcopy bytes data = 2;
```

Type attributes **must** precede the type. For arrays:

```plaintext
[] unsafe string names = 1;
[] rcopy bytes data = 2;
```

### Type Attributes

- **`unsafe`**: Uses the Go `unsafe` package for faster string ↔ byte slice conversions.
- **`rcopy`** (_Return Copy_): Allocates a **new buffer** and copies bytes from the source buffer instead of returning a reference (not cropped). ⚠️ **Includes memory allocations!**
  - This ensures that modifications to the original buffer (passed to `Unmarshal` functions) do not affect the unmarshalled data.

### Types

| **Benc**  | **Golang** |
| :-------: | :--------: |
|  `byte`   |   `byte`   |
|  `bytes`  |  `[]byte`  |
|   `int`   |   `int`    |
|  `int16`  |  `int16`   |
|  `int32`  |  `int32`   |
|  `int64`  |  `int64`   |
|  `uint`   |   `uint`   |
| `uint16`  |  `uint16`  |
| `uint32`  |  `uint32`  |
| `uint64`  |  `uint64`  |
| `float32` | `float32`  |
| `float64` | `float64`  |
|  `bool`   |   `bool`   |
| `string`  |  `string`  |
|   `[]T`   |   `[]T`    |
|  `[N]T`   |   `[N]T`   |
| `<K, V>`  | `map[K]V`  |

### Containers or Enums

A container or enum name refers to another defined structure.

**Container Example:**

```plaintext
ctr Person {
    int age = 1;
    string name = 2;
    Child child = 4;
}
```

Reference:

```plaintext
ctr Person2 {
    Person person = 1;
}
```

**Enum Example:**

```plaintext
enum JobStatus {
    Employed,
    Unemployed
}
```

Reference:

```plaintext
ctr Person {
    JobStatus jobStatus = 1;
}
```

### Fixed-Size Arrays

A field can be declared as a fixed-size array `[N]T` (where `N > 0`), which maps to a
Go array `[N]T`:

```plaintext
ctr Blob {
    [16]byte hash   = 1;
    [4]int32 vector = 2;
    [3]string names = 3;
}
```

The element `T` can be any supported type — primitives, `string`/`bytes`, enums,
containers, custom types, and nesting (`[][N]T`, `<K,[N]V>`). Compared to a slice `[]T`:

- It **decodes in place** (no allocation — the array is filled directly).
- It **validates length on decode**: a payload whose element count ≠ `N` returns
  `benc.ErrInvalidSize` (rather than panicking or corrupting the stream).
- `[N]byte` uses a single `copy()` instead of a per-element loop.

It reuses the slice wire format (so `[N]T` is interchangeable with `[]T` when the length
matches), which keeps fixed-array fields skippable for forward/backward compatibility.

> Note: `[N]int` / `[N]uint` (and `[]int` / `[]uint`) are not supported — use a sized
> integer such as `int32` / `int64`.

### Custom Types

Custom types let you use a named Go type as a field. They come in two forms.

> ⚠️ Custom types are **not** designed for backward/forward compatibility — they
> prioritize performance and simplicity. The codec writes raw value bytes; the
> per-field tag only locates the field by ID in a matched round-trip.

**Form A — newtype alias.** A named type over a scalar base type. `bencgen`
generates the Go `type` definition and a codec built on the `bstd` primitives:

```plaintext
custom Name   = string;
custom UserID = uint64;
```

This generates `type Name string` / `type UserID uint64` in the output package.
The base type may be any scalar: `string`, `bytes`, `bool`, `byte`,
`int`/`int16`/`int32`/`int64`, `uint`/`uint16`/`uint32`/`uint64`, `float32`/`float64`.

**Form B — external codec.** For a type `bencgen` can't generate (e.g.
`time.Time`), point at a package that provides the codec:

```plaintext
custom Timestamp {
    type   = "time.Time";                       # struct-field Go type
    import = "time";                            # OPTIONAL package providing `type`
    funcs  = "github.com/me/benccodecs";        # package exporting the codec funcs
}
```

The `funcs` package must export, named `Size<Name>`/`Marshal<Name>`/`Unmarshal<Name>`
(matching `bstd.SizeString` etc., so one package can back many custom types):

```go
func SizeTimestamp(v time.Time) int
func MarshalTimestamp(n int, b []byte, v time.Time) int
func UnmarshalTimestamp(n int, b []byte) (int, time.Time, error)
```

Custom types work as fields, array elements, and map keys/values:

```plaintext
ctr Invoice {
    UserID buyer = 1;
    Timestamp at = 2;
    []Name tags  = 3;
    <Name, UserID> idx = 4;
}
```

Custom map keys must be `comparable`. Custom types are local to the file that
declares them.

## Languages

Valid values for `--lang` are:

- `go`

## License

MIT

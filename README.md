[![GoDoc](https://godoc.org/github.com/Zaba505/sand?status.svg)](https://godoc.org/github.com/Zaba505/sand)
[![Go Report Card](https://goreportcard.com/badge/github.com/Zaba505/sand)](https://goreportcard.com/report/github.com/Zaba505/sand)
[![Build Status](https://travis-ci.com/Zaba505/sand.svg?branch=master)](https://travis-ci.com/Zaba505/sand)
[![Code Coverage](https://img.shields.io/codecov/c/github/Zaba505/sand/master.svg)](https://codecov.io/github/Zaba505/sand?branch=master)

# sand
`sand` is for creating interpreters, like the Python interpreter and Haskell interpreter.
It can also be used for creating text based games and CLI test environments.

For examples, check out the [examples](https://github.com/Zaba505/sand/tree/master/example) folder.

#### Design
`sand` implements a concurrent model. It views an interpreter as two seperate components:
the User Interface, `sand.UI`, and the Command Processor,`sand.Engine`. The following
diagram shows how under the hood `sand` operates. Every square is a goroutine.

```text
+--------+                            +--------------------------+
|        |              +------------->     Engines Manager      +--------------+
|  Read  <----------+   |             +--------------------------+              |
|        |          |   |                                                       |
+----+---+          |   |                                                       |
     |            +-+---+------+                                        +-------v------+
     |            |            |                                        |              |     +----------+
     +------------>     UI     |                                        |    Engine    |     |  Engine  |
                  |  (usually  +---------------------------------------->    Runner    +---->+   Exec   |
     +------------>    main)   |                                        |              |     |          |
     |            |            |      XXXXXXXXXXXXXXXXXXXXXXXXXXXX      |              |     +----------+
     |            +-+----------+      X   Manager connects UI    X      +--------------+
+----+---+          |                 X   to Engine Runner       X
|        |          |                 XXXXXXXXXXXXXXXXXXXXXXXXXXXX
| Write  <----------+
|        |
+--------+

```

`sand.UI` is a `struct` that is provided for you and is implemented as broad as possible;
however there are few features missing, which are commonly found in popular interpreters,
namely: Line history and Auto-completion. These features may be added later, but as for
now they are not planned for.

`sand.Engine` is an `interface`, which must be implemented by the user. Implementations
of `sand.Engine` must have a comparable underlying type, see [Go Spec](https://golang.org/ref/spec#Comparison_operators)
for comparable types in Go.
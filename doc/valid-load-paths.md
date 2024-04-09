# Load file naming convention

This document specifies the file names which may be loaded from Starlark.
The intention here is to enforce uniform, reasonable-looking, human-intelligible naming.

## Behaviour

Paths which start with `./` or `../` are interpreted relative to the current file.
Paths which do _not_ start with `./` or `../` are interpreted relative to the base of the extension directory.

Regardless of host OS, the `/` symbol is always interpreted as a path separator.

## Constraints

NB: Below, ‘path operator’ refers to either `./` or `../`.

| Valid `load` files...                                                                                  | This avoids loading...     |
| :------------------------------------------------------------------------------------------------------| :------------------------- |
| must have the `.star` file extension                                                                   | `i_am_banned.png`          |
| must contain only components with at least three characters                                            | `i/am/banned.star`         |
| must have a file stem with at least three characters                                                   | `banned/aa.star`           |
| must comprise only: unaccented latin lowercase letters, numbers, underscores, forward slashes and dots | `AAAA/-/🗑️/🔥.star`         |
| must only use dots in path operators and file extension delimiters                                     | `.i/a.m/.../banned.star.`  |
| must use zero or more path operators only at the start                                                 | `i/am/ok/.././banned.star` |
| must only use `./` at the very start                                                                   | `././banned.star`          |
| must not start with a slash                                                                            | `/i/am/banned.star`        |
| must not contain two or more successive slashes                                                        | `i//am///banned.star`      |
| must not contain two or more successive underscores                                                    | `i__am___banned.star`      |
| must have no components which start or end with an underscore                                          | `_i_/am_/_banned.star`     |
| must not contain an underscore followed by a dot                                                       | `banned_.star`             |

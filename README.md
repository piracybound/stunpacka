# stunpacka

stunpacka is a program for converting `.st` files to `.lua` files. 
fuck maintainers for being weird.

## Build

- [install go](https://golang.org/doc/install).

1. clone the repository.
2. build:
   ```bash
   go build .
   ```

## Usage

### GUI

1. run the program without arguments to enter interactive mode
2. drag and drop `.st` files onto the console window, or type filenames (can be processed in bulk)
3. available commands:
   - `help`: shows help dialogue
   - `credits`: shows credits
   - `website`: opens the discord server
   - `exit`: exits stunpacka

### CLI

process files via command line arguments with various options:

```bash
stunpacka [options] [files...]
```

options:
- `-out <directory>`: output directory for converted files
- `-verbose`: enable verbose output
- `-workers <num>`: maximum number of concurrent workers (default: CPU count)
- `-force`: force overwrite existing files
- `-help`: shows help dialogue

## example

### input
`.st` file

### output
```lua
-- manifest & lua provided by: https://www.piracybound.com/discord
-- via manilua
[decrypted lua here]
```

<sub>stunpacka © 2025 by piracybound is licensed under <a href="https://github.com/piracybound/stunpacka/blob/main/LICENSE">CC BY-ND 4.0</a></sub>
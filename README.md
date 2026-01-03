# kemono-dl
kemono-dl is a program for downloading videos, images and other files from kemono.party. You can build the executable yourself from the source code or download it from the releases page.

## Building the project:
### Clone the repository:

```bash
git clone https://github.com/VehovskyJ/kemono-dl
cd kemono-dl
```

### Build an executable:

To build an executable for your system use one of the following commands

```bash
make windows
make linux
make arm
make mac
```

## Usage

```bash
kemono-dl [options] <url>
```

## Options
`--force` Force update even if profile timestamp hasn't changed <br>
`--skip-download`Only fetch and save metadata, skip downloading files <br>
`--timeout` Set download timeout for each file (default: "2m"). Supports Go durations like `10s`, `5m`, etc.<br>
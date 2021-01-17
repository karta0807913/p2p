# p2p
send file in local area network, use udp broadcast, http download

## Install

```
go get github.com/karta0807913/p2p
```

## Usage

```
usage: p2p [-h|--help] -c|--code "<value>" [-o|--output_path "<value>"]
           [-s|--share_path "<value>"]

           A Simple Local Network Tool

Arguments:

  -h  --help         Print help information
  -c  --code         file code
  -o  --output_path  path for storage. Default: ./output
  -s  --share_path   which file want to share. Default:
```

## Example

Sender
```
$ p2p -c foo -s ./shared_file
```

Receiver
```
$ p2p -c foo -o ./save_path
```

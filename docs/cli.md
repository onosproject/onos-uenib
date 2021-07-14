# Command-Line Interface
The project provides a command-line facilities for remotely 
interacting with the UE NIB subsystem.

The commands are available at run-time using the consolidated `onos` client hosted in the `onos-cli` repository.

The documentation about building and deploying the consolidated `onos` client or its Docker container
is available in the `onos-cli` GitHub repository.

## Usage
To see the detailed usage help for the `onos uenib ...` family of commands,
please see the [CLI documentation](https://github.com/onosproject/onos-cli/blob/master/docs/cli/onos_uenib.md)

## Examples
Here are some concrete examples of usage:

List `CellInfo` of all UES.
```bash
$ onos uenib get ues --aspect onos.uenib.CellInfo
...
```

_MORE TODO_
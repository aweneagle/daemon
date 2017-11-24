# daemon

usage:

```golang
import (
  _ "github.com/aweneagle/daemon"
)

func main() {
    // do you jobs as usual
}

```

```sh

# start program front

./main

# start program as daemon

./main --daemon

# restart program

./main --signal restart

# shutdown program

./main --signal shutdown

```

* when start a program as an daemon, a directory name ".proc" will be created in the working directory, in which "./proc/sock" will be found

* supervise use --daemon, --signal stop|restart  as  command flag

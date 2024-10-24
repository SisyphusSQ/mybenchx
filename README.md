## mybenchx

mybenchx is a benchmark tool for MySQL, similar Sysbench.

In addition to real-time monitoring TPS, she also monitors vmstat/iostat via SSH tunnel.

Forked from [xelabs/benchyou](https://github.com/xelabs/benchyou)

Thanks For xelabs.

The idea of stat per operation is inspired by Mark Callaghan, [Small Datum](http://smalldatum.blogspot.com)

## Add feature
- Feature: `prepare` with `oltp-table-size` option to fill up tables.
- Feature: add `select` mode : `common`,`time_stamp`,`unix_stamp` 

## Bug fix
- Replace mysql driver from `go-mysqlstack` to `gorm` , due to fix `MySQL 8.0.x support`

## ToDo
- [x] mode select
- [x] Query efficiency contrast between `datetime` and `unix_stamp`
- [ ] XA support


## Screenshots
```
time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[1s]         [r:4,w:128,u:4,d:4]  33508    24056   9452    0      0.00     0      0.00      0.00    0.00      0.00    0.00     0.00    0       0         5.05       0.39         33508

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[2s]         [r:4,w:128,u:4,d:4]  29929    21287   8642    0      0.00     0      0.00      0.00    0.00      0.00    0.00     0.00    0       0         6.12       0.45         63437

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[3s]         [r:4,w:128,u:4,d:4]  27967    20215   7752    0      0.00     2472   0.09      0.00    0.00      25.57   0.94     6.22    6185    4570      6.51       0.51         91404

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[4s]         [r:4,w:128,u:4,d:4]  30072    21560   8512    0      0.00     2235   0.07      0.00    0.00      23.55   0.80     5.60    6174    4577      5.74       0.45         121476

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[5s]         [r:4,w:128,u:4,d:4]  32182    23609   8573    0      0.00     2810   0.09      0.00    0.00      29.55   0.94     5.91    6165    4584      5.45       0.46         153658

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[6s]         [r:4,w:128,u:4,d:4]  34548    24771   9777    0      0.00     2823   0.08      0.00    0.00      29.28   0.87     5.80    6156    4590      5.14       0.40         188206

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[7s]         [r:4,w:128,u:4,d:4]  35185    24844   10341   0      0.00     2553   0.07      0.00    0.00      26.40   0.77     5.74    6145    4596      5.20       0.38         223391

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[8s]         [r:4,w:128,u:4,d:4]  36266    26030   10236   0      0.00     2880   0.08      0.00    0.00      29.84   0.84     5.86    6137    4603      4.95       0.38         259657

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[9s]         [r:4,w:128,u:4,d:4]  37414    26834   10580   0      0.00     3234   0.09      0.00    0.00      34.07   0.93     6.06    6125    4611      4.81       0.37         297071

time            thds               tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op  freeMB  cacheMB   w-rsp(ms)  r-rsp(ms)    total-number
[10s]        [r:4,w:128,u:4,d:4]  36158    25845   10313   0      0.00     3329   0.09      0.00    0.00      35.53   1.01     6.43    6113    4619      4.98       0.38         333229

----------------------------------------------------------------------------------------------avg---------------------------------------------------------------------------------------------
time          tps     wtps    rtps    rio    rio/op   wio    wio/op    rMB     rKB/op    wMB     wKB/op   cpu/op            w-rsp(ms)                       r-rsp(ms)              total-number
[10s]        33642    24132   9509    0      0.00     332    0.00      0.00    0.00      3.55    0.01     0.07    [avg:0.53,min:0.00,max:149.79]  [avg:0.04,min:0.00,max:27.63]      336420
```

the columns:
```
time:         benchmark uptime
thds:         read threads and write(insert/update/delete) threads
tps:          transaction per second, including write and read
wtps:         write tps
rtps:         read tps
rio:          read io numbers per second
rio/op:       rio per operation
wio:          write io numbers per second
wio/op:       wio per operation
rMB:          amount data read from the device(megabytes) per second
rKB/op:       rKB per operation
wMB:          amount data written to the device(megabytes) per second
wKB/op:       wKB per operation
cpu/op:       CPU usecs per operation, measured by vmstat
freeMB:       the amount of idle memory(megabytes)
cacheMB:      the amount of memory(megabytes) used as cache
w-rsp:        the response time of one write operation,  in millisecond
r-rsp:        the response time of one read  operation,  in millisecond
total-number: the total number events
```

## Usage

```
$ ./bin/bench -h
Usage:
  mybenchx [command]

Available Commands:
  prepare     
  cleanup     
  random      
  seq         
  range       
  help        Help about any command
  completion  Generate the autocompletion script for the specified shell

Flags:
      --batch-per-commit int        #rows per transaction(Default 1) (default 1)
      --delete-threads int          number of delete threads to use(Default 0)
  -h, --help                        help for mybenchx
      --max-request uint            limit for total requests, including write and read(Default 0, means no limits)
      --max-time int                limit for total execution time in seconds(Default 3600) (default 3600)
      --mysql-db string             MySQL database name(Default sbtest) (default "sbtest")
      --mysql-enable-xa int         enable MySQL xa transaction for insertion {0|1} (Default 0)
      --mysql-host string           MySQL server host(Default NULL)
      --mysql-password string       MySQL password(Default mybenchx) (default "mybenchx")
      --mysql-port int              MySQL server port(Default 3306) (default 3306)
      --mysql-range-order string    range query sort the result-set in {ASC|DESC} (Default ASC) (default "ASC")
      --mysql-table-engine string   storage engine to use for the test table {tokudb,innodb,...} (default "innodb")
      --mysql-user string           MySQL user(Default mybenchx) (default "mybenchx")
      --oltp-table-size int         If not specify, will not fill up table 
      --oltp-tables-count int       number of tables to create(Default 8) (default 8)
      --query-type string           Query type which [common,time_stamp,unix_stamp] to use (default "common")
      --read-threads int            number of read threads to use(Default 32) (default 32)
      --rows-per-insert int         #rows per insert(Default 1) (default 1)
      --ssh-host string             SSH server host(Default NULL, same as mysql-host)
      --ssh-password string         SSH server password(Default mybenchx) (default "mybenchx")
      --ssh-port int                SSH server port(Default 22) (default 22)
      --ssh-user string             SSH server user(Default mybenchx) (default "mybenchx")
      --update-threads int          number of update threads to use(Default 0)
      --write-threads int           number of write threads to use(Default 32) (default 32)

Use "mybenchx [command] --help" for more information about a command.
```

## Examples

```
prepare 64 tables:
./bin/mybenchx  --mysql-host=10.3.2.1 --mysql-user=mybenchx --mysql-password=mybenchx  --oltp-tables-count=64 prepare

cleanup 64 tables:
./bin/mybenchx  --mysql-host=10.3.2.1 --mysql-user=mybenchx --mysql-password=mybenchx  --oltp-tables-count=64 cleanup

random insert(Write/Read Ratio=128:8):
 ./bin/mybenchx  --mysql-host=10.3.2.1 --mysql-user=mybenchx --mysql-password=mybenchx --ssh-user=mybenchx --ssh-password=mybenchx --oltp-tables-count=64 --write-threads=128 --read-threads=8 --max-time=3600 random

sequential insert(Write/Read Ratio=128:8):
 ./bin/mybenchx  --mysql-host=10.3.2.1 --mysql-user=mybenchx --mysql-password=mybenchx --ssh-user=mybenchx --ssh-password=mybenchx --oltp-tables-count=64 --write-threads=128 --read-threads=8 --max-time=3600 seq

mix(Write/Read/Update/Delete Ratio=4:4:4:4):
 ./bin/mybenchx  --mysql-host=10.3.2.1 --mysql-user=mybenchx --mysql-password=mybenchx --ssh-user=mybenchx --ssh-password=mybenchx --oltp-tables-count=64 --write-threads=4 --read-threads=4 --update-threads=4 --delete-threads=4 --max-time=3600 random

insert multiple rows(10 rows per insert):
 ./bin/mybenchx  --mysql-host=10.3.2.1 --mysql-user=mybenchx --mysql-password=mybenchx --ssh-user=mybenchx --ssh-password=mybenchx --oltp-tables-count=64 --write-threads=4 --rows-per-insert=10 --max-time=3600 random

batch update(10 rows per transaction):
 ./bin/mybenchx  --mysql-host=10.3.2.1 --mysql-user=mybenchx --mysql-password=mybenchx --ssh-user=mybenchx --ssh-password=mybenchx --oltp-tables-count=64 --update-threads=4 --batch-per-commit=10 --max-time=3600 random

query-range(Write/Read Ratio=128:8):
 ./bin/mybenchx  --mysql-host=10.3.2.1 --mysql-user=mybenchx --mysql-password=mybenchx --ssh-user=mybenchx --ssh-password=mybenchx --oltp-tables-count=64 --write-threads=128 --read-threads=8 --max-time=3600 --mysql-range-order=DESC range
```

## `Prepare` efficiency compare with `sysbench`
Performance Of VM   
CPU cores : 2  
Memory total : 4G

```bash
$ ./bench --mysql-host=127.0.0.1 --mysql-user=root --mysql-password=root \
> --oltp-tables-count=10 --oltp-table-size=100000 prepare
2022/03/04 17:46:20 create table mybenchx0(engine=innodb) finished...
2022/03/04 17:46:20 create table mybenchx1(engine=innodb) finished...
2022/03/04 17:46:21 create table mybenchx2(engine=innodb) finished...
2022/03/04 17:46:22 create table mybenchx3(engine=innodb) finished...
2022/03/04 17:46:22 create table mybenchx4(engine=innodb) finished...
2022/03/04 17:46:23 create table mybenchx5(engine=innodb) finished...
2022/03/04 17:46:24 create table mybenchx6(engine=innodb) finished...
2022/03/04 17:46:25 create table mybenchx7(engine=innodb) finished...
2022/03/04 17:46:25 create table mybenchx8(engine=innodb) finished...
2022/03/04 17:46:26 create table mybenchx9(engine=innodb) finished...
...
2022/03/04 17:49:36 [END] Table: mybenchx7 cost time: 189.574623194 sec
2022/03/04 17:49:36 [PROCESS] Table: mybenchx6 insert num: 99000 cost time: 189.829763464 sec
2022/03/04 17:49:37 [PROCESS] Table: mybenchx2 insert num: 99000 cost time: 190.861349855 sec
2022/03/04 17:49:38 [PROCESS] Table: mybenchx6 insert num: 100000 cost time: 191.215371057 sec
2022/03/04 17:49:38 [END] Table: mybenchx6 cost time: 191.215429047 sec
2022/03/04 17:49:38 [PROCESS] Table: mybenchx2 insert num: 100000 cost time: 191.89571655 sec
2022/03/04 17:49:38 [END] Table: mybenchx2 cost time: 191.895827813 sec

$ sysbench ./tests/include/oltp_legacy/oltp.lua --mysql-host=127.0.0.1 --mysql-port=3306 \
> --mysql-user=root --mysql-password=root --oltp-test-mode=complex --oltp-tables-count=10 \
> --oltp-table-size=100000 --threads=10 --time=120 --report-interval=10 prepare
sysbench 1.0.20 (using bundled LuaJIT 2.1.0-beta2)

Creating table 'sbtest1'...
Inserting 100000 records into 'sbtest1'
Creating secondary indexes on 'sbtest1'...
Creating table 'sbtest2'...
Inserting 100000 records into 'sbtest2'
Creating secondary indexes on 'sbtest2'...
Creating table 'sbtest3'...
Inserting 100000 records into 'sbtest3'
Creating secondary indexes on 'sbtest3'...
Creating table 'sbtest4'...
Inserting 100000 records into 'sbtest4'
Creating secondary indexes on 'sbtest4'...
Creating table 'sbtest5'...
Inserting 100000 records into 'sbtest5'
Creating secondary indexes on 'sbtest5'...
Creating table 'sbtest6'...
Inserting 100000 records into 'sbtest6'
Creating secondary indexes on 'sbtest6'...
Creating table 'sbtest7'...
Inserting 100000 records into 'sbtest7'
Creating secondary indexes on 'sbtest7'...
Creating table 'sbtest8'...
Inserting 100000 records into 'sbtest8'
Creating secondary indexes on 'sbtest8'...
Creating table 'sbtest9'...
Inserting 100000 records into 'sbtest9'
Creating secondary indexes on 'sbtest9'...
Creating table 'sbtest10'...
Inserting 100000 records into 'sbtest10'
Creating secondary indexes on 'sbtest10'...
Total Time: 416s
```
 
## License

mybenchx is released under the GPLv3. See LICENSE

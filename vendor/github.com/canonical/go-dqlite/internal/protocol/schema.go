package protocol

//go:generate ./schema.sh --request init

//go:generate ./schema.sh --request Leader    unused:uint64
//go:generate ./schema.sh --request Client    id:uint64
//go:generate ./schema.sh --request Heartbeat timestamp:uint64
//go:generate ./schema.sh --request Open      name:string flags:uint64 vfs:string
//go:generate ./schema.sh --request Prepare   db:uint64 sql:string
//go:generate ./schema.sh --request Exec      db:uint32 stmt:uint32 values:NamedValues
//go:generate ./schema.sh --request Query     db:uint32 stmt:uint32 values:NamedValues
//go:generate ./schema.sh --request Finalize  db:uint32 stmt:uint32
//go:generate ./schema.sh --request ExecSQL   db:uint64 sql:string values:NamedValues
//go:generate ./schema.sh --request QuerySQL  db:uint64 sql:string values:NamedValues
//go:generate ./schema.sh --request Interrupt db:uint64
//go:generate ./schema.sh --request Add       id:uint64 address:string
//go:generate ./schema.sh --request Assign    id:uint64 role:uint64
//go:generate ./schema.sh --request Remove    id:uint64
//go:generate ./schema.sh --request Dump      name:string
//go:generate ./schema.sh --request Cluster   format:uint64
//go:generate ./schema.sh --request Transfer   id:uint64

//go:generate ./schema.sh --response init
//go:generate ./schema.sh --response Failure  code:uint64 message:string
//go:generate ./schema.sh --response Welcome  heartbeatTimeout:uint64
//go:generate ./schema.sh --response NodeLegacy  address:string
//go:generate ./schema.sh --response Node   id:uint64 address:string
//go:generate ./schema.sh --response Nodes  servers:Nodes
//go:generate ./schema.sh --response Db       id:uint32 unused:uint32
//go:generate ./schema.sh --response Stmt     db:uint32 id:uint32 params:uint64
//go:generate ./schema.sh --response Empty    unused:uint64
//go:generate ./schema.sh --response Result   result:Result
//go:generate ./schema.sh --response Rows     rows:Rows
//go:generate ./schema.sh --response Files    files:Files

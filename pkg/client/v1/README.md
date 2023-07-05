## Tableland Client

Tableland Client is a convenient wrapper around validator's HTTP APIs. Here is the list of public APIs.

#### APIs
- [Create](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/writequery.go#L39)
- [Write](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/writequery.go#L73)
- [Version](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/version.go#L15)
- [GetTable](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/table.go#L19)
- [Receipt](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/receipt.go#L29)
- [Read](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/readquery.go#L64)
- [Validate](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/queryhelpers.go#L19)
- [Hash](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/queryhelpers.go#L10)
- [CheckHealth](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/health.go#L10)


### Usage
Following are few example usage of these APIs.

First, we instantiate the client: 

```go
import (

// ...
// ...

  clientV1 "github.com/textileio/go-tableland/pkg/client/v1"
  "github.com/textileio/go-tableland/pkg/wallet"
)

// create the wallet from the given private key
wallet, _ := wallet.NewWallet("PRIVATE_KEY")
ctx := context.Background()

// create the new client
client, _ := clientV1.NewClient(
    ctx, wallet, clientV1.[]NewClientOption{}...)
```

Checkout the available client options [here](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/client.go#L59).


##### Create
Create a new table with the schema and [options](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/writequery.go#L20).
Schema should be a valid SQL DDL as a string. 

```go
tableID, fullTableName := client.Create(
    ctx, "(id int, name text)",
    clientV1.WithPrefix("foo"))
```

##### Write
Write will execute an mutation query, returning the txn hash. Additional [options](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/writequery.go#L99) can be passed in.

```go
  // Create a new table
  tableID, fullTableName := client.Create(
    ctx, "(id int, name text)",
    clientV1.WithPrefix("foo"))

  // Update the table
  query := fmt.Sprintf(
    "update %s set name = alice where id = 1", fullTableName)
  hash := client.Write(ctx, query)

  // Alter table to rename a column
  query := fmt.Sprintf(
    "alter table %s rename name to username", fullTableName)
  hash := client.Write(ctx, query)
```


##### Receipt
Receipt will get the transaction receipt given the transaction hash. Additional configuration is possible with [options](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/receipt.go#L19).

```go
    txHash := "0xabcd"
    receipt, ok, err := client.Receipt(
      ctx, txnHash, clientV1.WaitFor(cp.receiptTimeout))
```

##### Read
Read runs a read SQL query with the provided [options](https://github.com/tablelandnetwork/go-tableland/blob/main/pkg/client/v1/readquery.go#L35).

```go
    // Create a new table: `myTable`
    client.Create(ctx, "(counter int)", clientV1.WithPrefix("myTable"))

    // Insert a row in `myTable`
    client.write(ctx, "insert into myTable (counter) values(42)")

    // Read it into the `target` struct
    result := map[string]interface{}{}
    client.Read(
        ctx, "select counter from myTable",
        result, clientV1.ReadExtract())
```

##### GetTable
The GetTable API will return the [Table](https://github.com/tablelandnetwork/go-tableland/blob/ac993505b32ccd32ad0c7b3d9552b14c0eb72823/internal/router/controllers/apiv1/model_table.go#L12) struct given the [table id](https://github.com/tablelandnetwork/go-tableland/blob/ac993505b32ccd32ad0c7b3d9552b14c0eb72823/pkg/client/v1/client.go#L206). 

```go
    // create a new table 
    tableID, fullTableName := client.Create(
        ctx, "(bar text DEFAULT 'foo',zar int, CHECK (zar>0))",
	    WithPrefix("foo"), WithReceiptTimeout(time.Second*10))
	
    // get the table using tableID
    table, err := client.GetTable(ctx, tableID)
```


# coral-importer

![Test](https://github.com/coralproject/coral-importer/workflows/Test/badge.svg)

Visit the [Releases](https://github.com/coralproject/coral-importer/releases) page
to download a release of the `coral-importer` tool.

## Strategies

### CSV

When importing vis CSV, each column must be provided, even if it is optional. In
the case of the `data/csv/users.csv` file, you would at minimum want the
something like following:

```csv
1,example@example.com,example,,,
```

Where you notice that the fields for `role`, `banned`, and `created_at` are not
provided. If you attempted to provide a CSV file without those fields provided,
it would error.

```sh
# This now provides the export files that can be processed by the importer.
TENANT_ID=c2440817-464e-4a8f-8851-24effd8fee9d
SITE_ID=3f183f3d-205f-41da-881a-e5089057b78f
INPUT=data/csv
OUTPUT=database

coral-importer --quiet csv --input $INPUT --tenantID $TENANT_ID --siteID $SITE_ID --output $OUTPUT 2> output.log
```

#### Encoding

The importer is built on GoLang which prefers UTF-8 encoded CSV text files. If you find yourself with a `Windows 1252` or `ISO 88592` encoded CSV file, we kindly suggest you pre-convert the file using your preferred tool. If your files are too big to open in an editor, we suggest using `iconv`.

More details about `iconv` available [here](https://linux.die.net/man/1/iconv).

_Example conversion from Windows 1252:_

```
iconv -f WINDOWS-1252 -t UTF8 < users.csv > users-clean.csv
```

_Example conversion from ISO 88592:_

```
iconv -f ISO88592 -t UTF8 < users.csv > users-clean.csv
```

#### Format

`data/csv/users.csv`:

| #   | Column     | Type    | Required | Description                                                                                        |
| --- | ---------- | ------- | -------- | -------------------------------------------------------------------------------------------------- |
| 0   | id         | string  | yes      | User's ID.                                                                                         |
| 1   | email      | string  | yes      | Email address of the User.                                                                         |
| 2   | username   | string  | yes      | Username of the User.                                                                              |
| 3   | role       | string  | no       | Role of the User, can be one of `COMMENTER`, `ADMIN`, or `MODERATOR` (Default `COMMENTER`).        |
| 4   | banned     | boolean | no       | Can be one of `true` or `false` (Default `false`).                                                 |
| 5   | created_at | string  | no       | [ISO8601](http://en.wikipedia.org/wiki/ISO_8601) formatted date string (Defaults to current date). |

`data/csv/stories.csv`:

| #   | Column       | Type   | Required | Description                                                                                                                            |
| --- | ------------ | ------ | -------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| 0   | id           | string | Yes      | ID of the Story.                                                                                                                       |
| 1   | url          | string | Yes      | URL of the Story.                                                                                                                      |
| 2   | title        | string | No       | Title of the Story (will be scraped on next visit).                                                                                    |
| 3   | author       | string | No       | Author of the Story (will be scraped on next visit).                                                                                   |
| 4   | published_at | string | No       | Publish date of the Story as a [ISO8601](http://en.wikipedia.org/wiki/ISO_8601) formatted date string (will be scraped on next visit). |
| 5   | closed_at    | string | No       | Date as a [ISO8601](http://en.wikipedia.org/wiki/ISO_8601) formatted date string when commenting was closed (Default is unset).        |
| 6   | mode         | string | No       | Story mode, can be one of `COMMENTS`, `QA`, or `RATINGS_AND_REVIEWS` (Default `COMMENTS`)                                              |

`data/csv/comments.csv`:

| #   | Column     | Type   | Required | Description                                                                                                    |
| --- | ---------- | ------ | -------- | -------------------------------------------------------------------------------------------------------------- |
| 0   | id         | string | Yes      | ID of the Comment.                                                                                             |
| 1   | author_id  | string | Yes      | ID of the User that authored the Comment.                                                                      |
| 2   | story_id   | string | Yes      | ID of the Story that this Comment was written on.                                                              |
| 3   | created_at | string | Yes      | Date as a [ISO8601](http://en.wikipedia.org/wiki/ISO_8601) formatted date string when the Comment was written. |
| 4   | body       | string | Yes      | Comment with limited formatting HTML. Non-formatting HTML will be removed on import.                           |
| 5   | parent_id  | string | No       | ID of the Comment that this is a reply to (Default to unset, indicating that this is not a reply).             |
| 6   | status     | string | No       | Status of the Comment, can be one of `APPROVED`, `REJECTED`, or `NONE` (Default's to `NONE`).                  |
| 7   | rating     | number | No       | Rating attached to the Comment                                                                                 |

### Legacy

```sh
DATABASE_NAME=coral
MONGO_CONTAINER_ID=mongo-export

# Make the export directory.
mkdir -p "export"

# Dump each collection to the export directory. This operation can take some
# time for larger data sets.
collections=(actions assets comments settings users)
for collection in ${collections[*]}
do
  docker run --rm -ti --link $MONGO_CONTAINER_ID:mongo-export -v $PWD/export:/mnt/export mongo:4.2 mongoexport --host mongo-export -d $DATABASE_NAME -c $collection  -o /mnt/export/${collection}.json
done

# This now provides the export files that can be processed by the importer.
TENANT_ID=c2440817-464e-4a8f-8851-24effd8fee9d
SITE_ID=3f183f3d-205f-41da-881a-e5089057b78f
INPUT=data/legacy
OUTPUT=database

coral-importer --quiet legacy --input $INPUT --tenantID $TENANT_ID --siteID $SITE_ID --output $OUTPUT 2> output.log
```

### Livefyre

```sh
# Perform the import parsing operation.
TENANT_ID=c2440817-464e-4a8f-8851-24effd8fee9d
SITE_ID=3f183f3d-205f-41da-881a-e5089057b78f
USER_INPUT=data/livefyre/users.json
COMMENTS_INPUT=data/livefyre/comments.json
OUTPUT=database

coral-importer --quiet livefyre --users $USER_INPUT --comments $COMMENTS_INPUT --tenantID $TENANT_ID --siteID $SITE_ID --output $OUTPUT 2> output.log
```

## Importing

```sh
# Upload the generated imports to MongoDB.
TARGET_MONGO_CONTAINER=mongo
DATABASE_NAME=coral
CONCURRENCY=$(sysctl -n hw.ncpu)

collections=(commentActions stories users comments)
for collection in ${collections[*]}
do
  if [ ! -f $PWD/$OUTPUT/$collection.json ]
  then
    echo "$PWD/$OUTPUT/$collection.json does not exist, not importing $collection collection"
    continue
  fi

  docker run --rm -ti -v $PWD:/mnt/import --link mongo:$TARGET_MONGO_CONTAINER mongo:4.2 mongoimport --uri=mongodb://mongo/$DATABASE_NAME --file=/mnt/import/$OUTPUT/$collection.json --collection $collection --numInsertionWorkers $CONCURRENCY
done
```

If you're updating documents:

```sh
collections=(commentActions stories users comments)
for collection in ${collections[*]}
do
  if [ ! -f $PWD/$OUTPUT/$collection.json ]
  then
    echo "$PWD/$OUTPUT/$collection.json does not exist, not importing $collection collection"
    continue
  fi

  docker run --rm -ti -v $PWD:/mnt/import --link mongo:$TARGET_MONGO_CONTAINER mongo:4.2 mongoimport --uri=mongodb://mongo/$DATABASE_NAME --file=/mnt/import/$OUTPUT/$collection.json --collection $collection --numInsertionWorkers $CONCURRENCY --mode upsert --upsertFields id
done
```

## Tricks

Print all the active operations on your database with messages.

```js
db.currentOp()
  .inprog.filter((op) => Boolean(op.msg))
  .map((op) => ({ ns: op.ns, msg: op.msg, command: op.command }));
```

## Benchmarks

|                    |       |
| ------------------ | ----- |
| Running importer   | 1m30s |
| Importing Comments | 4m30s |
| Importing Stories  | 4s    |
| Importing Users    | 30s   |
| Rebuilding Indexes | 20m   |

## Developing

```sh
collections=(actions assets comments settings users)
for collection in ${collections[*]}
do
  head ${collection}.json > ${collection}_sample.json
  head -n1 ${collection}.json > ${collection}_single.json
done
```

To update models run:
```sh
go generate ./...
```

To locally build an executable for testing use the output flag:
```sh
go build -o <location>
```

## Changelog

### v0.4.13
- (coral) handles `metadata.displayName` values if present on legacy users

### v0.4.3

- (coral) monotonic timestamps will no longer reset and cause a conflict if more
  than 1000 are created in the same second.

### v0.4.1

- (coral) `createdAt` timestamps that are used by Coral as cursors now are
  emitted as unique timestamps when not disabled with the
  `--disableMonotonicCursorTimes` flag. This means that every timestamp emitted
  that shares the same second time will automatically have it's ms time
  incremented to prevent collisions.

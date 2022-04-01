# coral-importer

![Test](https://github.com/coralproject/coral-importer/workflows/Test/badge.svg)

Visit the [Releases](https://github.com/coralproject/coral-importer/releases) page
to download a release of the `coral-importer` tool.

## Strategies


### Legacy

This import strategy is designed to migrate data from Coral ^4.12.0 to
^6.17.1.

This strategy requires you to stop any Coral instances that are
interacting with the MongoDB database that Coral uses. The first step to
this process is to dump the files from the remote MongoDB database to
your local machine to perform the migration:

```bash
#!/bin/bash

# If this script errors, stop.
set -e

# Set this to the TALK_MONGO_URL used by Coral.
export TALK_MONGO_URL="..."

# Set this to a folder where we'll export the documents from your ^4 database.
export CORAL_INPUT_DIRECTORY="$PWD/coral/input"

# Set this to a folder where we'll export the documents to be uploaded to your
# ^6 database.
export CORAL_OUTPUT_DIRECTORY="$PWD/coral/output"

# Make the directories used by this and the following tools.
mkdir -p "${CORAL_INPUT_DIRECTORY}" "${CORAL_OUTPUT_DIRECTORY}"

# Dump each collection to the export directory. This operation can take some
# time for larger data sets.
collections=(actions assets comments settings users)
for collection in ${collections[*]}
do
  mongoexport --uri "$TALK_MONGO_URL" -c $collection -o "${CORAL_INPUT_DIRECTORY}/${collection}.json"
done

# Set this to the ID of your Tenant. If you're importing from a ^4 instance
# and do not have a Tenant ID, generate one using `uuidgen`.
export CORAL_TENANT_ID="c2440817-464e-4a8f-8851-24effd8fee9d"

# Set this to the ID of your Site. If you're importing from a ^4 instance
# and do not have a Site ID, generate one using `uuidgen`.
export CORAL_SITE_ID="3f183f3d-205f-41da-881a-e5089057b78f"

# This importer tool is designed to work with Coral at thw following migration
# version. This is the newest file in the
# https://github.com/coralproject/talk/tree/develop/src/core/server/services/migrate/migrations
# directory for your version of Coral.
export CORAL_MIGRATION_ID="1582929716101"

# Set this to the file location where you want to export your log files to.
export CORAL_LOG="$PWD/coral/logs.json"

# Run the importer tool in dry mode to perform document validation before we
# actually write any files. This may take some time and will use about 40% of
# the dataset's size in RAM to perform the validation.
coral-importer legacy --dryRun

# If the previous command completed successfully, then you can run it for real.
coral-importer legacy

# This should write all the output files to the `$CORAL_OUTPUT_DIRECTORY`
# directory. If you did not use SSO with your ^4 instance of Coral using
# plugins, you can continue below to the *Importing* section. Otherwise.

# Set this to the name of a directory we can write output files that have
# been mapped.
export CORAL_MAPPER_POST_DIRECTORY="$PWD/coral/post"

mkdir -p "${CORAL_MAPPER_POST_DIRECTORY}"

# If your custom SSO plugin saved the User ID in the Users `profiles` array
# as `profiles.id`, like the following:
#
# {
#   "profiles": [
#     {
#       "provider": "my-auth",
#       "id": "..."
#     }
#   ]
# }
#
# Then you should set the following so the mapper can grab the `profiles.id`
# from the profile and map it to a corresponding SSO profile for ^6.
# export CORAL_MAPPER_USERS_SSO_PROVIDER="my-auth"

# If a custom plugin wrote a users username to a field other than `username`,
# such as:
#
# {
#   "metadata": {
#     "displayName": "My Name"
#   }
# }
#
# Then you should set the following so the mapper can grab the username from
# the other field.
# export CORAL_MAPPER_USERS_USERNAME="metadata.displayName"

# Run the importer tool in dry mode to perform document validation before we
# actually write any files.
coral-importer legacy --dryRun map

# If the previous command completed successfully, then you can run it for real.
coral-importer legacy map
```

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

**`data/csv/users.csv`**:

| #   | Column     | Type    | Required | Description                                                                                        |
| --- | ---------- | ------- | -------- | -------------------------------------------------------------------------------------------------- |
| 0   | id         | string  | yes      | User's ID.                                                                                         |
| 1   | email      | string  | yes      | Email address of the User.                                                                         |
| 2   | username   | string  | yes      | Username of the User.                                                                              |
| 3   | role       | string  | no       | Role of the User, can be one of `COMMENTER`, `ADMIN`, or `MODERATOR` (Default `COMMENTER`).        |
| 4   | banned     | boolean | no       | Can be one of `true` or `false` (Default `false`).                                                 |
| 5   | created_at | string  | no       | [ISO8601](http://en.wikipedia.org/wiki/ISO_8601) formatted date string (Defaults to current date). |

**`data/csv/stories.csv`**:

| #   | Column       | Type   | Required | Description                                                                                                                            |
| --- | ------------ | ------ | -------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| 0   | id           | string | Yes      | ID of the Story.                                                                                                                       |
| 1   | url          | string | Yes      | URL of the Story.                                                                                                                      |
| 2   | title        | string | No       | Title of the Story (will be scraped on next visit).                                                                                    |
| 3   | author       | string | No       | Author of the Story (will be scraped on next visit).                                                                                   |
| 4   | published_at | string | No       | Publish date of the Story as a [ISO8601](http://en.wikipedia.org/wiki/ISO_8601) formatted date string (will be scraped on next visit). |
| 5   | closed_at    | string | No       | Date as a [ISO8601](http://en.wikipedia.org/wiki/ISO_8601) formatted date string when commenting was closed (Default is unset).        |
| 6   | mode         | string | No       | Story mode, can be one of `COMMENTS`, `QA`, or `RATINGS_AND_REVIEWS` (Default `COMMENTS`)                                              |

**`data/csv/comments.csv`**:

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

## Importing

```sh
# Set this to the MONGO_URL used by the new Coral ^6 instance. This should be
# different than the CORAL_MONGO_URI used by ^4.
export MONGO_URL="..."

# Set this to a folder where we'll export the documents to be uploaded to your
# ^5 database.
export CORAL_OUTPUT_DIRECTORY="$PWD/coral/output"

# This command should get the number of CPU's available on your machine,
# otherwise if it fails just set it to the number of CPU's manually.
export CONCURRENCY="$(sysctl -n hw.ncpu)"

# For each of these collections, import them into the new MongoDB database.
collections=(commentActions stories users comments)
for collection in ${collections[*]}
do
  mongoimport --uri "$MONGO_URL" --file "${CORAL_OUTPUT_DIRECTORY}/$collection.json" --collection "$collection" --numInsertionWorkers $CONCURRENCY
done
```

If you're updating documents:

```sh

# For each of these collections, import them into the new MongoDB database.
collections=(commentActions stories users comments)
for collection in ${collections[*]}
do
  mongoimport --uri "$MONGO_URL" --file "${CORAL_OUTPUT_DIRECTORY}/$collection.json" --collection "$collection" --numInsertionWorkers $CONCURRENCY --mode upsert --upsertFields id
done
```
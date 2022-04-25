
# Intro

The purpose of the diagnostics script is to provide a secure and trusted method
of gathering and uploading logs on systems that are experiencing issues.

# Method

The following steps are taken for creating diagnostic logs:

1. Log data is separated into various categories, the standard output and error
output are recorded for analysis by developers.

2. Compress and encrypt the log data directory using a randomly generated or
user provided AES key.

3. Encrypt the AES key "metadata" using a developer provided public key.

4. Upload the symmetric encrypted log data and public key encrypted metadata
for remote analysis.

# Implementation Details

The `diagnostics.sh` command can take an optional argument which describes the
desired steps to take. By default the script will perform a
`gather-upload-confirm`, where the steps of uploading and including metadata
are confirmed by prompt. If the argument is given as `gather` the encryption
and upload step will be skipped. An argument of `gather-upload` would upload
without prompt and include metadata for decryption, while 
`gather-upload-nometa` would upload without prompting and without including
necessary decryption metadata.

The user must then communicate the ID of the log file uploaded for analysis, 
and if the encryption metadata was not included in the upload that information
must also be communicated in a secure manner.

AES encryption metadata will appear in a form like the following:
```
Save secret metadata for log decryption:
salt=62e2ac13ae6bb66e
key=375dc2863c5b340252c0e5c631dda24b4fdc343139410b97fb5b7678919d8752
iv=7874c21533a1b4a4129c00e95bd9d0e4
```

The "salt", "key", and "iv" are randomly generated values using openssl, or
they can be manually passed in as environment variables. Similarly a "UUID" is
auto-generated if not provided through the environment.

# Decrypting Data

When decrypting log data for analysis a developer requires the following
information:

1. A working `gsutil` command setup with credentials using the appropriate
bucket. `pip install gsutil` should install and 
`echo -e 'rancher-dev-file.json\nk3s-diagnostic-logs' | gsutil config -e` to
configure access.

2. The AES encryption metadata if not included in the upload (defined as
environment variables), or the private key and passphrase for decrypting
metadata if included in upload.

The UUID of the desired log set should also be provided (either as env or
first argument), and may be partial or contain wildcards.

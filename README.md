# Hero-file

## Status

in development but operational for testing

Caution: some flags, methods etc. may change until stable release

## About

*hero-file* is an Openstack Swift client equivalent (though not all features are implemented).

It can be used to upload/download/list some swift bucket.

Program does not create buckets automatically, it only access existing buckets

## License

Apache 2.0, see LICENSE file.

## Authentication

A token can be given via --os-auth-token option or Openstack credentials file can be used with following env variables: *OS_AUTH_URL, OS_USER_DOMAIN_ID, OS_PROJECT_DOMAIN_ID, OS_PROJECT_NAME, OS_USERNAME, OS_PASSWORD*. It supports keystone auth v3 only.

## Running

    # export HERO_DEBUG=1 // for debug
    export HEROTOKEN=XXX
    
    go run hero-file.go --os-auth-token $HEROTOKEN --os-storage-url https://genostack-api-swift.genouest.org/v1/AUTH_XXX list mybucketname


With openstack credentials file:

    . ~/my_openstackrc.sh
    go run hero-gile.go list mybucketname


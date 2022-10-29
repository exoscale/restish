Here is a small guide on how to get set-up for using our own fork
of restish with publi-api. To summarize:

- The patches haven't been reviewed upstream, you will need to install
  a fork of restish
- A companion script has to be installed
- Restish profiles have to be created
- Public API needs to run with a recent (at least 1.0.0-alpha158)
  version

Note that the following has only been tested on Linux, a few minor
may need to be applied on MacOS.

## Installing restish and the companion script

You will first need to build a patched version of restish:

``` shell
git clone -b bundle git@github.com:exoscale/restish
cd restish && go build
```

Once built, install it at a location in your PATH.

``` shell
sudo cp restish /usr/local/bin/
```

If you already have restish installed, you may either uninstall it,
or ensure that the newly built one takes precedence.

## Installing the authentication helper

The helper script is also present in the branch:

``` shell
sudo cp docs/restish-exoscale-auth /usr/local/bin
```

## Creating restish profiles

With the above handled, new profiles have to be created. The profiles
can be created with the help of `restish api configure exo` but here
is a JSON excerpt that can be directly used in `$HOME/.restish/apis.json`:

The configuration below assumes that a functional `$HOME/.cloudstack.in`
file exists with the first profile targetting production and a `preprod`
profile targetting preprod.

``` json
{
  [...]
  "exo": {
    "base": "https://api-ch-gva-2.exoscale.com/v2",
    "profiles": {
      "default": {
        "base": "",
        "auth": {
          "name": "external-tool",
          "params": {
            "commandline": "restish-exoscale-auth"
          }
        }
      },
      "preprod": {
        "base": "https://ppapi-ch-gva-2.exoscale.com/v2",
        "auth": {
          "name": "external-tool",
          "params": {
            "commandline": "restish-exoscale-auth -p preprod"
          }
        }
      }
    },
    "tls": {
      "insecure": false,
      "cert": "",
      "key": "",
      "ca_cert": ""
    }
  }
}
```

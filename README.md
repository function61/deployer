[![Build Status](https://img.shields.io/travis/function61/deployer.svg?style=for-the-badge)](https://travis-ci.org/function61/deployer)
[![Download](https://img.shields.io/bintray/v/function61/dl/deployer.svg?style=for-the-badge&label=Download)](https://bintray.com/function61/dl/deployer/_latestVersion#files)

Deploy anything with this container-based deployment tool. It supports using different
container images to contain the deployment tooling.

![](docs/hold-on-to-your-butts.gif)


How does it work?
-----------------

See example project, [Onni](https://github.com/function61/onni), using Deployer (it also has
instructions).

Basically, deploying anything is downloading a zip archive with deployment support files
(not necessarily the application itself!) that will be mounted inside the deployment tooling
image and deployment scripts will be invoked inside the container.

In Onni's case [this directory](https://github.com/function61/onni/tree/master/deployerspec)
is zipped (along with a generated `version.json` file) at build-time and uploaded as a build
artefact to Bintray which is downloadable at `"https://dl.bintray.com/function61/dl/onni/$version/deployerspec.zip"`.

Therefore when you run `$ deployer deploy onni https://url/to/the.zip`, it will download and unzip
the zip file and read the
[manifest.json](https://github.com/function61/onni/blob/master/deployerspec/manifest.json)
to find out which `deployer_image` to use, ask you about user-specific deployment settings
(like API credentials), inject them and finally hand off the dirty work to the container.

In Onni's case the container image contains [Terraform](https://www.terraform.io/) which
ultimately takes care of the heavy lifting to call all the relevant AWS APIs.

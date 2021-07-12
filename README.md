# RPG Maker VX Ace Loader

⚠️ *This is still a work in-progress and backwards compatibility is not guaranteed*

[![Actions Status](https://github.com/silbinarywolf/rmvx/workflows/Go/badge.svg)](https://github.com/silbinarywolf/rmvx/actions)

A Go library that can load RPG Maker VX Ace data files by decoding Ruby Marshal files.

## Credits

- [Ancurio](https://github.com/Ancurio/mkxp) for their open-source RPG Maker XP / VX Ace implementation.
- [Dozen](https://github.com/dozen/ruby-marshal) for their incomplete Ruby marshal decoder implementation. This was used as the primary reference for this implementation.
- [nsf](https://github.com/nsf/jsondiff) for their JSON diff library. This is used for tests only and is embedded to avoid external dependencies.
- [Ruby Documentation](https://docs.ruby-lang.org/en/2.1.0/marshal_rdoc.html) for high-level information on the Ruby marshal file format.
- [Ruby Marshal Implementation in C](https://github.com/ruby/ruby/blob/e330bbeeb1bd70180e5f6b835f2a39488e6c2d42/marshal.c) used as the canonical source for how it works.

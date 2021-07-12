# Ruby Marshal Decoder

A decoder for Ruby Marshal files, created specifically to decode RPG Maker VX Ace files.

This is not it's own package by design for two reasons:

- It's easier to maintain if I bundle it with the RPG Maker VX Ace data loading code.
- I'd rather write tests at a higher-level to fit my use-case than make a stable Ruby marshal decoder.

## Credits

- [Dozen](https://github.com/dozen/ruby-marshal) for their incomplete Ruby marshal decoder implementation. This was used as the primary reference for this implementation.
- [Ruby Documentation](https://docs.ruby-lang.org/en/2.1.0/marshal_rdoc.html) for high-level information on the Ruby marshal file format.
- [Ruby Marshal Implementation in C](https://github.com/ruby/ruby/blob/e330bbeeb1bd70180e5f6b835f2a39488e6c2d42/marshal.c) used as the canonical source for how it works.

# Craftspeople we learn from

The catalog daemon is closer to a Unix utility than to a web service,
and we want it built the way the best Unix software is. These are the
authors whose work we want to emulate. Most engineers, and most language
models, already know their ideas; this list is a reminder to start from
them.

## W. Richard Stevens

_Advanced Programming in the UNIX Environment_ (Addison-Wesley, 1992;
the third edition, with Stephen A. Rago, came out in 2013), and _UNIX
Network Programming_. Stevens covers the facilities the daemon is built
from: files and directories, signals, daemon processes, file and record
locking, and interprocess communication through pipes, FIFOs, and Unix
domain sockets.

What we take from him: before inventing a protocol, use what the
operating system already gives every process. Two programs that can see
the same folder already share a channel.

## Eric S. Raymond

_The Art of Unix Programming_ (Addison-Wesley, 2003). Raymond gathers
the Unix philosophy into a set of rules, among them modularity, clarity,
composition, separation, simplicity, transparency, and least surprise.
He also gives a chapter to how Unix programs cooperate across processes.

What we take from him: keep the design simple enough that a person can
understand what the daemon is doing by looking at the files, and prefer
plain text and ordinary files to hidden state.

## Using this list

When a design question involves processes, files, or how programs
coordinate, start from these books' answers before reaching for newer
protocols such as MCP. When someone else's work shapes a design here,
add them with the book or project and what we take from it.

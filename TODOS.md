# Trkr

## TODO 
1. look for proc by name [x]
2. look for proc by id [x]
3. get a proc start time [x]
4. start session tracking a task  [x]
5. end a session [x]
6. save session data  [x]
7. save sessions [x]
8. take cli args [x]
9. filter system processes out and show user processes only [-]
10. allow user to chose and specify what to track [x] // kinda
11. generate a report [-]
12. abstract the platform layer [x]
13. make a detection loop that runs every 500ms * 4 [x]
14. make a poll for /proc/{pid}/stat that runs every 500ms [x]
15. make the main thread handle spwaning threads and saving to db [x]
16. gracefully handle signal interrupts, faults and other shinanigins [x]
17. periodically save and update session data to db [x]
18. use queue channel to put db actions on a queue and handle in separate thread




## Bugs 

1. fix restarting session for Procs that are in the cleaning up process by OS // add a delay [x]


## Questions
1. how can I track that I have a window open [x]
2. how do I know what do I want to track [x]
3. what are the boundaries that I can draw [-]
4. how do I know that a session is active vs not [x]
5. what is the best way to deal with db []
6. what are the session boundaries ?  // a session boundary should be defined by a process's pid and start time // (guaranteed by the OS to be unique)



## Structure
      server  // asks engine for reports
        ^ ^ \
        |  \ \
        |   \ \
        |    \ \
        |     \ \
        |      \ \
        |       \ v    // should it save locally ?
        |      engine (rerpots to server every heartbeat)
        |
     client (should it pull or server should push?)



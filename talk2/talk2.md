# Distributed Tracing
 - Show Activity
 - Talk about diagnostic source and Activity
 - Talk about w3c trace context
 - Talk about correlated log scopes using trace id
 - Talk about open telemetry
 - Talk about other zipkin, and app insights
 - Show demo of seq and zipkin
 
# VS exception rethrow indicator
 - Show MVC application working with synchronous code
 - Show it broken when the code is changed to async
 - Break on all exceptions
 - Show how the stack trace is shown for every exception thrown in the async chain
 
# VS parallel tasks
 - Start dotnet counters
 - Show lots of blocking requests, show parallel stacks
 - Flip the code to do a real async call and show that there's still a hang but less threads are being used
 - Attach debugger look at parallel tasks
 - Collect a dump using dotnet dump collect and show dump async -stacks
 
# Memory Leak
 - Controller action storing things in a dictionary and never letting go
 - Hammer with requests
 - Collect a full dump with dotnet dump collect talk about platform dependance
 - Show dump analysis using dotnet dump analyze
 - Show opening the same dump in Visual Studio
 - Collect a gc dump
   - Discuss platform independence
   - Discuss size difference
   - Discuss not pausing the app
 - Show new VS allocation profiler

# App is running slowly
- CPU is high
- Show counters
- Show dotnet trace collecting a CPU profile
- Open the profile in VS
- Open the profile in perfview
 
# App is running slowly
- ThreadPool starvation
- Show counters

# App is crashing
 - Capture dump on crash
 
 Backuo dem
 - Show Micronetes

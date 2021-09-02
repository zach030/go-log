# go-log
> Make it easy to write log and get log stream

## **How to Log in**

The log is naturally separated by custom field, such ad "app_id, task_id, topic" etc.

```go
Instance().LogIn("task_id","msg")
// It's quite easy to stop log in
Instance().Stop("task_id")
```
## **How to persist Log**

The log file is temporarily store in disk where you can modify the path.

It's recommend to upload file timely and remove from disk.

## **How to get Log stream**

As mentioned above, it's easy to get log stream.
```go
Instance().Stream("task_id")
```
It returns ``stream chan string`` and support concurrently stream.

Here's the concept about ``dead log`` and ``active log``

- dead log: The input stream is closed And the specific task_id is removed. So the log stream is from log file.
- active log: The specific task_id is still active in memory, the log input stream is still open, so it's real time log flow.
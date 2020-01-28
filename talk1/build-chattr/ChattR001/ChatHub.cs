using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Microsoft.AspNetCore.SignalR;

namespace ChattR001
{
    public class ChatHub : Hub
    {
        public Task Send(string message)
        {
            return Clients.All.SendAsync("Send", message);
        }

        public async IAsyncEnumerable<DateTime> Stream()
        {
            for (int i = 0; i < 10; i++)
            {
                yield return DateTime.UtcNow;

                await Task.Delay(1000);
            }
        }
    }
}
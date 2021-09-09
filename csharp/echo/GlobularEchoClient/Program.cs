using System;

namespace Globular
{
    class Program
    {
        static void Main(string[] args)
        {
            string send_message = "Hello World!";
            Console.WriteLine("send message " + send_message);
            EchoClient client = new EchoClient("b776ef9b-7d89-4b76-ab1b-6246c68692b4", "localhost");
            string receive_message = client.Echo(send_message);
             Console.WriteLine("received message " + receive_message);
        }
    }
}

using System;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using OpenTelemetry;
using OpenTelemetry.Trace.Configuration;

namespace FrontEnd
{
    public class Startup
    {
        // This method gets called by the runtime. Use this method to add services to the container.
        // For more information on how to configure your application, visit https://go.microsoft.com/fwlink/?LinkID=398940
        public void ConfigureServices(IServiceCollection services)
        {
            services.AddOpenTelemetry(builder =>
            {
                builder.UseZipkin(o =>
                {
                    o.Endpoint = new Uri("http://localhost:9411/api/v2/spans");
                    o.ServiceName = typeof(Startup).Assembly.GetName().Name;
                });

                builder.AddRequestCollector()
                       .AddDependencyCollector();
            });
        }

        // This method gets called by the runtime. Use this method to configure the HTTP request pipeline.
        public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
        {
            if (env.IsDevelopment())
            {
                app.UseDeveloperExceptionPage();
            }

            app.UseRouting();

            app.UseEndpoints(endpoints =>
            {
                var client = new HttpClient
                {
                    BaseAddress = new Uri("https://localhost:5001")
                };

                endpoints.MapGet("/", async context =>
                {
                    var content = await client.GetStringAsync("/");

                    await context.Response.WriteAsync(content);
                });
            });
        }
    }
}

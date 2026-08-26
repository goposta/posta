---
sidebar_position: 5
title: .NET
description: Posta .NET SDK
---

# .NET SDK

Official typed .NET client for Posta.

## Installation

```shell
dotnet add package Posta
```

**Requires:** .NET 10+

## Quick Start

```csharp
using Posta.Clients;
using Posta.Models.Emails;

using var client = new PostaClient(
    "https://posta.example.com",
    "your-api-key");

SendAnEmailResponse? response = await client.Emails.SendAnEmailAsync(
    new SendAnEmailRequest
    {
        From = "Acme <hello@example.com>",
        To = ["user@example.com"],
        Subject = "Hello from Posta",
        Html = "<h1>Hello!</h1>"
    });
```

The client also provides typed API clients for templates, campaigns, subscribers,
webhooks, workspaces, administration, and the other Posta API areas.

## Aspire Integration

For applications using .NET Aspire, install the client integration package:

```shell
dotnet add package Posta.Aspire
```

Register the client using the Posta resource name from your AppHost:

```csharp
builder.AddPostaClient("posta", settings =>
{
    settings.ApiKey = builder.Configuration["Posta:ApiKey"];
});
```

Posta also has an official hosting integration in the
[Aspire Community Toolkit](https://github.com/CommunityToolkit/Aspire/tree/main/src/CommunityToolkit.Aspire.Hosting.Posta).

## Error Handling

Non-successful HTTP responses throw `PostaApiException`, which exposes the HTTP
status code, response body, and structured Posta error details.

```csharp
using Posta.Transport;

try
{
    await client.Emails.GetEmailDetailsAsync(
        new GetEmailDetailsRequest { Id = emailId });
}
catch (PostaApiException exception)
{
    Console.WriteLine(exception.StatusCode);
    Console.WriteLine(exception.Error?.Message);
}
```

See the [Posta .NET SDK repository](https://github.com/goposta/posta-dotnet) for
complete configuration, Aspire, and API coverage documentation.

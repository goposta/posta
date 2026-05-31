---
sidebar_position: 4
title: Java
description: Posta Java SDK
---

# Java SDK

Official Java client for Posta.

## Installation

### Maven (JitPack)

```xml
<repositories>
    <repository>
        <id>jitpack.io</id>
        <url>https://jitpack.io</url>
    </repository>
</repositories>

<dependency>
    <groupId>com.github.goposta</groupId>
    <artifactId>posta-java</artifactId>
    <version>v1.1.0</version>
</dependency>
```

### Gradle

```groovy
repositories {
    maven { url 'https://jitpack.io' }
}

dependencies {
    implementation 'com.github.goposta:posta-java:v1.1.0'
}
```

**Requires:** Java 11+

## Quick Start

```java
import com.github.goposta.posta.PostaClient;
import com.github.goposta.posta.SendEmailRequest;
import com.github.goposta.posta.SendResponse;

PostaClient client = new PostaClient("https://posta.example.com", "your-api-key");

SendResponse response = client.sendEmail(new SendEmailRequest()
    .from("sender@example.com")
    .to(List.of("recipient@example.com"))
    .subject("Hello from Posta")
    .html("<h1>Hello!</h1><p>This is a test email.</p>"));

System.out.printf("Email sent: id=%s status=%s%n", response.getId(), response.getStatus());
```

## Send Template Email

```java
SendResponse response = client.sendTemplateEmail(new SendTemplateEmailRequest()
    .template("welcome")
    .to(List.of("user@example.com"))
    .from("noreply@example.com")
    .language("en")
    .templateData(Map.of("name", "Alice")));
```

## Batch Send

```java
BatchResponse response = client.sendBatch(new BatchRequest()
    .template("newsletter")
    .from("news@example.com")
    .recipients(List.of(
        new BatchRecipient().email("user1@example.com").templateData(Map.of("name", "Bob")),
        new BatchRecipient().email("user2@example.com").language("fr").templateData(Map.of("name", "Carol"))
    )));

System.out.printf("Sent: %d, Failed: %d%n", response.getSent(), response.getFailed());
```

## Check Delivery Status

```java
EmailStatusResponse status = client.getEmailStatus("email-uuid");
System.out.printf("Status: %s%n", status.getStatus());
```

## Error Handling

```java
import com.github.goposta.posta.PostaException;

try {
    client.getEmailStatus("invalid-uuid");
} catch (PostaException e) {
    System.out.printf("Status: %d%n", e.getStatusCode());
    System.out.printf("Message: %s%n", e.getMessage());
}
```

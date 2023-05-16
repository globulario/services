# Globular Services
Globular services are gRPC services with predefined attributes that make them manageable, each services can be use without Globular. ATTOW Golang, C++, Typescript, and C# contain code for writting Globular services. 

Currently, there are 28 available microservices that can be used by your applications. Here is a list of some of the most useful ones:

* File Service: This service can be used for file operations such as creating, renaming, moving, and deleting files or directories.

* Event Service: This service acts as an event hub, allowing different parts of your application to communicate with each other. You can create event channels and propagate events on the network to synchronize clients or multiple servers. It follows the pub-sub principle and can be used to create an event-driven architecture. Globular itself makes use of this service for inter-service communications.

* Persistence: This service provides an interface to MongoDB (with support for other stores in the future). It gives your web application access to a persistence store. The API is simple, offering CRUD operations and covering almost all the functionality offered by MongoDB. It is secure and easy to use.

* SQL: This service helps you connect your web application to a SQL server. Connection information is hidden on the server side. By using stored procedures on the SQL server, no SQL queries are visible on the client side, making it more difficult for malicious attacks. The API is simple and provides all you need to interact with a SQL server from your web application.

* LDAP: This service is used to connect with an LDAP server. By doing so, you can make use of users already defined in LDAP in your application and keep them synchronized. It can also be connected to the authentication server to authenticate users.

* Mail: This microservice implements the SMTP and IMAP protocols. You can configure your own SMTP server and IMAP server or set up connection info for existing ones like Gmail. By doing so, you will be able to send and access your email from your applications.

* Log: A simple logging service used to report errors with severity levels.

* Monitoring: This service gives you access to a time series database (currently Prometheus). This allows you to access time series data and display it in your applications.

* RBAC: This service can be used to validate resource access, such as files, conversations, title info, etc. Its role is to protect your information.

* Storage: This is essentially a key-value store that can be used as a cache or persistent cache. Currently, BigCache and Badger are used to implement this service.

* Other services such as Authentication, Configuration, Resource, and Repository are also available...

Note: Some services may be further developed or enhanced in the future.

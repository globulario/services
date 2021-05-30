# Resource service
The resource service contain globular entities definitions. Those entities are use by other services. For exemple RBAC (Role Base Access Control) need entities like Role, Group, Application, Session etc... All those entities must be store and retreive. At this time Globular made use of **mongoDB** to achives this goal, but any other document database and maybe **SQL** can implemented the datastore interfaces (**Go**).

## Entities
There a list of all entities type definied by the resource service,

### Account
Most of the time you need User's Management System in your application, Globular take care of it.  There's the list of fields that compose an account,

 * **id** The account id, it must be unique.
 * **name** The account name can be the same as the id.
 * **email** The account email
 * **password** The account password (encrypted values).

 Reference to other Entities
 * **contacts** Each account can store reference to other account(s). 
 * **organizations** Organization is a mix betheen a Group and a Role, it help to manage Account access.
 * **groups** Group are use to manage more than one account at time.
 * **roles** Action's permission can not be granted to account directly, role are the way do to it... 


package catalog_client

import (
	"log"
	"testing"
	"time"

	"github.com/globulario/services/golang/catalog/catalogpb"
	"github.com/globulario/services/golang/testutil"
	Utility "github.com/globulario/utility"
	"google.golang.org/protobuf/encoding/protojson"
)

// newTestClient creates a client for testing, skipping if external services are not available.
func newTestClient(t *testing.T) *Catalog_Client {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	addr := testutil.GetAddress()
	client, err := NewCatalogService_Client(addr, "catalog_server")
	if err != nil {
		t.Fatalf("NewCatalogService_Client: %v", err)
	}
	return client
}

func TestCreatePersistenceConnection(t *testing.T) {
	client := newTestClient(t)
	log.Println("test create connection.")
	err := client.CreateConnection("catalogue_2_db", "catalogue_2_db", "localhost", float64(27017), float64(0), "sa", "adminadmin", float64(0), "")
	if err != nil {
		log.Println("fail to create connection ", err)
	}
}

// First test create a fresh new connection...
func TestSaveUnitMeasure(t *testing.T) {
	client := newTestClient(t)
	log.Println("test create unit of measure.")
	client.SaveUnitOfMesure("catalogue_2_db", "INCH", "en", "inch", `″`, "The inch (abbreviation: in or ″) is a unit of length in the (British) imperial and United States customary systems of measurement")
	client.SaveUnitOfMesure("catalogue_2_db", "INCH", "fr", "pouce", `″`, "Le pouce (symbole : ″ (double prime) ou po au Canada francophone) est une unité de longueur datant du Moyen Âge.")

	client.SaveUnitOfMesure("catalogue_2_db", "EACH", "en", "each", `″`, "Unitary quantity.")
	client.SaveUnitOfMesure("catalogue_2_db", "EACH", "fr", "chacun", `″`, "Mesure unitaire.")
}

// Create some common properties.
func TestSavePropertyDefintion(t *testing.T) {
	client := newTestClient(t)
	// Length
	client.SavePropertyDefinition("catalogue_2_db", "LENGTH", "en", "length", `l`, "Length is commonly understood to mean the most extended dimension of an object.", 3.0)
	client.SavePropertyDefinition("catalogue_2_db", "LENGTH", "fr", "longueur", `l`, "La longueur est une grandeur physique et une dimension spatiale. C'est une unité fondamentale dans pratiquement tout système d'unités. C'est notamment la dimension fondamentale unique du système d'unités géométriques, qui présente la singularité de ne pas avoir d'autre unité fondamentale.", 3.0)
	// Diameter
	client.SavePropertyDefinition("catalogue_2_db", "DIAMETER", "en", "diameter", `∅`, "In geometry, a diameter of a circle is any straight line segment that passes through the center of the circle and whose endpoints lie on the circle. It can also be defined as the longest chord of the circle. Both definitions are also valid for the diameter of a sphere.", 3.0)
	client.SavePropertyDefinition("catalogue_2_db", "DIAMETER", "fr", "diamètre", `∅`, "Dans un cercle ou une sphère, le diamètre est un segment de droite passant par le centre et limité par les points du cercle ou de la sphère. Le diamètre est aussi la longueur de ce segment.", 3.0)
}

func TestSaveItemDefintion(t *testing.T) {
	client := newTestClient(t)
	// External properties.
	properties_ids_en := &catalogpb.References{
		Values: []*catalogpb.Reference{
			{
				RefDbName: "catalogue_2_db",
				RefObjId:  Utility.GenerateUUID("LENGTHen"),
				RefColId:  "PropertyDefinition",
			},
			{
				RefDbName: "catalogue_2_db",
				RefObjId:  Utility.GenerateUUID("DIAMETERen"),
				RefColId:  "PropertyDefinition",
			},
		},
	}

	properties_ids_fr := &catalogpb.References{
		Values: []*catalogpb.Reference{
			{
				RefDbName: "catalogue_2_db",
				RefObjId:  Utility.GenerateUUID("LENGTHfr"),
				RefColId:  "PropertyDefinition",
			},
			{
				RefDbName: "catalogue_2_db",
				RefObjId:  Utility.GenerateUUID("DIAMETERfr"),
				RefColId:  "PropertyDefinition",
			},
		},
	}

	properties_ids_en_str, _ := protojson.Marshal(properties_ids_en)
	properties_ids_fr_str, _ := protojson.Marshal(properties_ids_fr)

	// Create item definition from predefined properties.
	client.SaveItemDefinition("catalogue_2_db", "PIPE", "en", "pipe", ``, `A pipe is a tubular section or hollow cylinder, usually but not necessarily of circular cross-section, used mainly to convey substances which can flow — liquids and gases (fluids), slurries, powders and masses of small solids. It can also be used for structural applications; hollow pipe is far stiffer per unit weight than solid members.`, "", string(properties_ids_en_str))
	client.SaveItemDefinition("catalogue_2_db", "PIPE", "fr", "pipe", ``, `Un tuyau est un élément de section circulaire destiné à l'écoulement d'un fluide, liquide, ou gaz ou d'un solide pulvérulent, au transport de l'énergie de pression (air comprimé, vapeur, huile hydromécanique, etc.), à l'échange de l'énergie au travers de la paroi (échangeur thermique, radiateur). Il peut être rigide ou souple (flexible). La paroi du tuyau sépare l'intérieur de l'extérieur et permet ces fonctions.`, "", string(properties_ids_fr_str))

}

// Test save item instance function.
func TestSaveItemInstance(t *testing.T) {
	client := newTestClient(t)

	// Here I will create french item instance.
	pipe_instance := &catalogpb.ItemInstance{
		ItemDefinitionId: "PIPE",
		Id:               "instance_0",
		Values: []*catalogpb.PropertyValue{
			{
				PropertyDefinitionId: "DIAMETER",
				Value: &catalogpb.PropertyValue_DimensionVal{
					DimensionVal: &catalogpb.Dimension{
						UnitId: "INCH",
						Value:  1.01,
					},
				},
			},
			{
				PropertyDefinitionId: "LENGTH",
				Value: &catalogpb.PropertyValue_DimensionVal{
					DimensionVal: &catalogpb.Dimension{
						UnitId: "INCH",
						Value:  20.0,
					},
				},
			},
		},
	}

	pipe_instance_str, _ := protojson.Marshal(pipe_instance)

	client.SaveItemInstance("catalogue_2_db", string(pipe_instance_str))
}

// Test save a manufacturer
func TestSaveManufacturer(t *testing.T) {
	client := newTestClient(t)
	client.SaveManufacturer("catalogue_2_db", "3M", "3M corporation")
}

func TestSavePackage(t *testing.T) {
	client := newTestClient(t)

	err := client.SavePackage("catalogue_2_db", "pipe_pack_1", "pipe six pack", "en", "package of six pipe", nil)

	if err != nil {
		log.Println(err)
	}

	err = client.SavePackage("catalogue_2_db", "pipe_pack_1", "tuyaux packet de six", "fr", "paquet de six tuyaux", nil)

	if err != nil {
		log.Println(err)
	}
}

// save/create a new supplier.
func TestSaveSupplier(t *testing.T) {
	client := newTestClient(t)
	client.SaveSupplier("catalogue_2_db", "Fastenal", "Fastenal")
}

func TestSavePackageSupplier(t *testing.T) {
	client := newTestClient(t)

	// Set the package reference.
	packageRef := &catalogpb.Reference{
		RefDbName: "catalogue_2_db",
		RefObjId:  Utility.GenerateUUID("pipe_pack_1fr"),
		RefColId:  "Package",
	}

	// Set the supplier ref.
	supplierRef := &catalogpb.Reference{
		RefDbName: "catalogue_2_db",
		RefObjId:  Utility.GenerateUUID("Fastenal"),
		RefColId:  "Supplier",
	}

	price := &catalogpb.Price{
		Value:    99.99,
		Currency: catalogpb.Currency_can,
	}

	price_str, _ := protojson.Marshal(price)
	packageRef_str, _ := protojson.Marshal(packageRef)
	supplierRef_str, _ := protojson.Marshal(supplierRef)

	err := client.SavePackageSupplier("catalogue_2_db", "000123254", string(supplierRef_str), string(packageRef_str), string(price_str), time.Now().Unix(), 1)
	if err != nil {
		log.Println(err)
	}
}

func TestSaveItemManufacturer(t *testing.T) {
	client := newTestClient(t)
	// Set the package reference.
	itemRef := &catalogpb.Reference{
		RefDbName: "catalogue_2_db",
		RefObjId:  Utility.GenerateUUID("instance_0"),
		RefColId:  "ItemInstance",
	}

	// Set the supplier ref.
	manufacturerRef := &catalogpb.Reference{
		RefDbName: "catalogue_2_db",
		RefObjId:  Utility.GenerateUUID("3M"),
		RefColId:  "Manufacturer",
	}

	itemRef_str, _ := protojson.Marshal(itemRef)
	manufacturerRef_str, _ := protojson.Marshal(manufacturerRef)

	err := client.SaveItemManufacturer("catalogue_2_db", "3M_011002", string(manufacturerRef_str), string(itemRef_str))
	if err != nil {
		log.Println(err)
	}
}

// save/create a new supplier.
func TestSaveCategory(t *testing.T) {
	client := newTestClient(t)
	client.SaveCategory("catalogue_2_db", "Pipes", "Tuyaux", "fr", "")
}

/*
func TestAppendItemdescriptionCategory(t *testing.T) {
	client.SaveCategory("catalogue_2_db", "Pipes", "Tuyaux", "fr", "")
	itemDefinitionRef := &catalogpb.Reference{
		RefDbName: "catalogue_2_db",
		RefObjId:  Utility.GenerateUUID("PIPEfr"),
		RefColId:  "ItemDefinition",
	}

	categoryRef := &catalogpb.Reference{
		RefDbName: "catalogue_2_db",
		RefObjId:  Utility.GenerateUUID("Pipesfr"),
		RefColId:  "Category",
	}

	var marshaler jsonpb.Marshaler
	itemDefinition_str, _ := marshaler.MarshalToString(itemDefinitionRef)
	categoryRef_str, _ := marshaler.MarshalToString(categoryRef)

	err := client.AppendItemDefinitionCategory("catalogue_2_db", itemDefinition_str, categoryRef_str)
	if err != nil {
		log.Println(err)
	}
}

func TestRemoveItemdescriptionCategory(t *testing.T) {
	client.SaveCategory("catalogue_2_db", "Pipes", "Tuyaux", "fr", "")
	itemDefinitionRef := &catalogpb.Reference{
		RefDbName: "catalogue_2_db",
		RefObjId:  Utility.GenerateUUID("PIPEfr"),
		RefColId:  "ItemDefinition",
	}

	categoryRef := &catalogpb.Reference{
		RefDbName: "catalogue_2_db",
		RefObjId:  Utility.GenerateUUID("Pipesfr"),
		RefColId:  "Category",
	}

	var marshaler jsonpb.Marshaler
	itemDefinition_str, _ := marshaler.MarshalToString(itemDefinitionRef)
	categoryRef_str, _ := marshaler.MarshalToString(categoryRef)

	err := client.RemoveItemDefinitionCategory("catalogue_2_db", itemDefinition_str, categoryRef_str)
	if err != nil {
		log.Println(err)
	}
}
*/

func TestSaveLocalisation(t *testing.T) {
	client := newTestClient(t)
	mag0 := new(catalogpb.Localisation)
	mag0.Id = "P001"
	mag0.LanguageCode = "fr"
	mag0.Name = "Magasin"

	row0 := new(catalogpb.Localisation)
	row0.Id = "P001_r0"
	row0.Name = "Rangé 0"
	row0.LanguageCode = "fr"

	loc0 := new(catalogpb.Localisation)
	loc0.Id = "P001_r0_loc0"
	loc0.Name = "localisation 0"
	loc0.LanguageCode = "fr"

	mag0.SubLocalisations = new(catalogpb.References)
	mag0.SubLocalisations.Values = append(mag0.SubLocalisations.Values, &catalogpb.Reference{RefDbName: "catalogue_2_db", RefColId: "Localisation", RefObjId: row0.Id + row0.LanguageCode})

	row0.SubLocalisations = new(catalogpb.References)
	row0.SubLocalisations.Values = append(row0.SubLocalisations.Values, &catalogpb.Reference{RefDbName: "catalogue_2_db", RefColId: "Localisation", RefObjId: loc0.Id + loc0.LanguageCode})

	client.SaveLocalisation("catalogue_2_db", loc0)
	client.SaveLocalisation("catalogue_2_db", row0)
	client.SaveLocalisation("catalogue_2_db", mag0)
}

func TestSaveInventory(t *testing.T) {
	client := newTestClient(t)

	inventory := new(catalogpb.Inventory)
	inventory.LocalisationId = "loc0"
	inventory.PacakgeId = "pipe_pack_1"
	inventory.SafetyStock = 10
	inventory.Reorderquantity = 7
	inventory.Quantity = 8
	inventory.Factor = 1.0

	_ = client // TODO: uncomment when SaveInventory is implemented
	//client.SaveInventory("catalogue_2_db", inventory)
	_ = inventory
}

package server

import (
	"context"
	"log/slog"

	"sweveh/internal/audit"
	"sweveh/internal/catalog"
	catalogphotos "sweveh/internal/catalog/adapters/out/photos"
	catalogpg "sweveh/internal/catalog/adapters/out/postgres"
	catalogapp "sweveh/internal/catalog/app"
	"sweveh/internal/customers"
	customerspg "sweveh/internal/customers/adapters/out/postgres"
	customersapp "sweveh/internal/customers/app"
	"sweveh/internal/inventory"
	inventoryphotos "sweveh/internal/inventory/adapters/out/photos"
	inventorypg "sweveh/internal/inventory/adapters/out/postgres"
	inventoryapp "sweveh/internal/inventory/app"
	"sweveh/internal/orders"
	"sweveh/internal/orders/adapters/out/directory"
	orderspg "sweveh/internal/orders/adapters/out/postgres"
	"sweveh/internal/orders/adapters/out/quotesdir"
	orderssettings "sweveh/internal/orders/adapters/out/settings"
	ordersstorage "sweveh/internal/orders/adapters/out/storage"
	ordersapp "sweveh/internal/orders/app"
	"sweveh/internal/outbox"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/platform/storage"
	"sweveh/internal/quotes"
	quotesdeps "sweveh/internal/quotes/adapters/out/deps"
	quotespg "sweveh/internal/quotes/adapters/out/postgres"
	quotesapp "sweveh/internal/quotes/app"
	"sweveh/internal/tenancy"
	tenancypg "sweveh/internal/tenancy/adapters/out/postgres"
	"sweveh/internal/vehicles"
	vehiclespg "sweveh/internal/vehicles/adapters/out/postgres"
	vehiclesapp "sweveh/internal/vehicles/app"
)

// Modules son los servicios de negocio ya cableados. Los comparten el api y el worker.
type Modules struct {
	Tx     *db.TxManager
	Audit  audit.Log
	Events outbox.Publisher
	Clock  clock.Clock
	Log    *slog.Logger

	Workshops tenancy.Workshops
	Admin     *tenancy.Admin
	Catalog   *catalogapp.Service
	CatalogF  catalog.Catalog
	Customers *customersapp.Service
	CustDir   customers.Directory
	Vehicles  *vehiclesapp.Service
	VehDir    vehicles.Directory
	Orders    *ordersapp.Service
	Flow      *ordersapp.Flow
	Findings  *ordersapp.Findings
	OrdersF   orders.Orders
	Inventory *inventoryapp.Service
	InvF      inventory.Inventory
	Quotes    *quotesapp.Quotes
	QuotesF   quotes.Quotes

	// PlatformAdmin dice si quien llama es administrador del ERP (HU-74, ADR-0030). Lo pone New; nil = nadie lo es.
	PlatformAdmin func(ctx context.Context, p reqctx.Principal) (bool, error)
}

// Build arma los módulos sobre una conexión (rol sweveh_api o sweveh_worker).
func Build(tx *db.TxManager, st storage.Store, clk clock.Clock, log *slog.Logger) *Modules {
	m := &Modules{Tx: tx, Audit: audit.NewLog(), Events: outbox.NewPublisher(), Clock: clk, Log: log}
	m.Workshops = tenancy.NewWorkshops(tenancypg.NewRepository(), m.Audit)
	m.Catalog = catalogapp.NewService(tx, catalogpg.Repository{}, m.Audit, clk)
	m.Catalog.SetPriceList(catalogphotos.New(st), catalogpg.Proposals{}) // HU-93; el lector lo pone wire si hay IA
	m.CatalogF = catalog.New(m.Catalog)
	m.Admin = tenancy.NewAdmin(tx, m.CatalogF, m.Audit, clk)

	m.Customers = customersapp.NewService(tx, customerspg.NewRepository(), m.Audit, m.Events, clk)
	m.Customers.SetExtra(customerspg.Extra{})
	m.CustDir = customers.NewDirectory(m.Customers)
	m.Vehicles = vehiclesapp.NewService(tx, vehiclespg.NewRepository(), m.CustDir, m.Audit, m.Events, clk)
	m.Vehicles.SetExtra(vehiclespg.Extra{})
	m.Vehicles.SetCustomerRegistrar(m.CustDir) // importación de clientes y vehículos (HU-92)
	m.VehDir = vehicles.NewDirectory(m.Vehicles)

	vdir, cdir := directory.NewVehicles(m.VehDir), directory.NewCustomers(m.CustDir)
	m.Orders = ordersapp.NewService(tx, orderspg.NewRepository(), vdir, cdir, m.Audit, m.Events, clk)
	flowRepo := orderspg.Flow{}
	m.Orders.SetExampleMarker(flowRepo.MarkExample)
	m.Flow = ordersapp.NewFlow(tx, flowRepo, m.Orders, vdir, ordersstorage.New(st), m.Audit, m.Events, clk)
	m.Flow.SetStats(orderspg.Stats{}) // cifras del tablero (HU-68 a HU-73) y de los avisos programados
	m.OrdersF = orders.New(m.Flow)
	m.Flow.SetSettings(orderssettings.Reader{A: m.Admin})
	m.Findings = ordersapp.NewFindings(tx, flowRepo, orderspg.Findings{}, m.Audit, clk)

	m.Inventory = inventoryapp.NewService(tx, inventorypg.Repository{}, m.Audit, m.Events, clk)
	m.Inventory.SetPhotoStore(inventoryphotos.New(st))
	m.InvF = inventory.New(m.Inventory)
	m.Flow.SetPartsReturner(m.InvF) // HU-26: cancelar devuelve a bodega lo descontado
	m.Quotes = quotesapp.NewService(tx, quotespg.Repository{}, quotesdeps.Orders{O: m.OrdersF}, quotesdeps.Catalog{C: m.CatalogF},
		quotesdeps.Inventory{I: m.InvF}, m.Audit, m.Events, clk)
	m.QuotesF = quotes.New(m.Quotes)
	m.Quotes.SetRules(quotesdeps.Rules{A: m.Admin})
	m.Flow.SetQuoteReader(quotesdir.Reader{Q: m.QuotesF})
	return m
}

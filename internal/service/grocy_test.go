package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/androidand/spisordning/internal/grocy"
	"github.com/androidand/spisordning/internal/service"
)

// newGrocyTestServer stands up a fake Grocy API and returns a service pointed
// at it.
func newGrocyTestServer(t *testing.T) (*service.Grocy, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/system/info", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"4.6.0"}`))
	})
	mux.HandleFunc("GET /api/objects/products", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"objects":[
			{"id":1,"name":"Milk","barcode":"739","location_id":1,"qu_id_stock":2,"qu_id_purchase":3,"min_stock_amount":1},
			{"id":2,"name":"Bread","barcode":"740","location_id":1,"qu_id_stock":4,"qu_id_purchase":4,"min_stock_amount":0}
		]}`))
	})
	mux.HandleFunc("GET /api/objects/stock", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"objects":[
			{"id":10,"product_id":1,"amount":2,"qu_id":2,"location_id":1,"best_before":"2026-09-01"},
			{"id":11,"product_id":2,"amount":0,"qu_id":4,"location_id":1,"best_before":""},
			{"id":12,"product_id":2,"amount":3,"qu_id":4,"location_id":1,"best_before":""}
		]}`))
	})
	mux.HandleFunc("GET /api/objects/shopping_list", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"objects":[
			{"id":100,"product_id":1,"note":"","amount":1,"qu_id":3,"done":false,"shopping_list_id":1},
			{"id":101,"product_id":0,"note":"napkins","amount":1,"qu_id":0,"done":false,"shopping_list_id":1}
		]}`))
	})
	mux.HandleFunc("POST /api/stock/products/1/add", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true}`))
	})
	mux.HandleFunc("POST /api/stock/products/1/consume", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true}`))
	})
	mux.HandleFunc("POST /api/stock/shoppinglist/add-product", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := grocy.New(srv.URL, "test-key")
	return service.NewGrocy(client, srv.URL), srv
}

func TestGrocyStatus_Configured(t *testing.T) {
	svc, _ := newGrocyTestServer(t)
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured || !status.Reachable {
		t.Fatalf("expected configured+reachable, got %+v", status)
	}
}

func TestGrocyStatus_NotConfigured(t *testing.T) {
	svc := service.NewGrocy(nil, "")
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Configured || status.Reachable {
		t.Fatalf("expected not configured, got %+v", status)
	}
}

func TestGrocyListProducts(t *testing.T) {
	svc, _ := newGrocyTestServer(t)
	products, err := svc.ListProducts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 2 || products[0].Name != "Milk" {
		t.Fatalf("unexpected products: %+v", products)
	}
}

func TestGrocyListStock_FiltersZeroAndNames(t *testing.T) {
	svc, _ := newGrocyTestServer(t)
	stock, err := svc.ListStock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Zero-amount lot (id 11) should be filtered out; 2 lots remain.
	if len(stock) != 2 {
		t.Fatalf("expected 2 non-zero lots, got %d: %+v", len(stock), stock)
	}
	// Product names should be enriched.
	byID := make(map[int]string)
	for _, e := range stock {
		byID[e.ProductID] = e.ProductName
	}
	if byID[1] != "Milk" || byID[2] != "Bread" {
		t.Fatalf("unexpected product names: %+v", byID)
	}
}

func TestGrocyListShoppingList(t *testing.T) {
	svc, _ := newGrocyTestServer(t)
	items, err := svc.ListShoppingList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[1].ProductID != 0 || items[1].Note != "napkins" {
		t.Fatalf("expected free-text item, got %+v", items[1])
	}
}

func TestGrocyAddStock(t *testing.T) {
	svc, _ := newGrocyTestServer(t)
	if err := svc.AddStock(context.Background(), 1, 2, "2026-09-01"); err != nil {
		t.Fatal(err)
	}
}

func TestGrocyAddStock_BadDate(t *testing.T) {
	svc, _ := newGrocyTestServer(t)
	if err := svc.AddStock(context.Background(), 1, 2, "not-a-date"); err == nil {
		t.Fatal("expected error for bad best_before")
	}
}

func TestGrocyAddStock_NotConfigured(t *testing.T) {
	svc := service.NewGrocy(nil, "")
	if err := svc.AddStock(context.Background(), 1, 2, ""); !errors.Is(err, service.ErrGrocyNotConfigured) {
		t.Fatalf("expected ErrGrocyNotConfigured, got %v", err)
	}
}

func TestGrocyConsumeStock(t *testing.T) {
	svc, _ := newGrocyTestServer(t)
	if err := svc.ConsumeStock(context.Background(), 1, 1); err != nil {
		t.Fatal(err)
	}
}

func TestGrocyAddShoppingItem(t *testing.T) {
	svc, _ := newGrocyTestServer(t)
	if err := svc.AddShoppingItem(context.Background(), 0, "napkins", 1); err != nil {
		t.Fatal(err)
	}
}

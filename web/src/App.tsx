import { NavLink, Route, Routes } from "react-router-dom";
import PlannerPage from "./pages/PlannerPage";
import ShoppingPage from "./pages/ShoppingPage";
import ComparePage from "./pages/ComparePage";
import RecipesPage from "./pages/RecipesPage";
import RecipeFamilyPage from "./pages/RecipeFamilyPage";
import PreferencesPage from "./pages/PreferencesPage";
import PantryPage from "./pages/PantryPage";
import PricesPage from "./pages/PricesPage";
import StoreLocatorPage from "./pages/StoreLocatorPage";
import BarcodePage from "./pages/BarcodePage";
import DashboardPage from "./pages/DashboardPage";
import AliasesPage from "./pages/AliasesPage";
import InspirationPage from "./pages/InspirationPage";
import GrocyPage from "./pages/GrocyPage";
import OrdersPage from "./pages/OrdersPage";
import SyncPage from "./pages/SyncPage";
import TonightPage from "./pages/TonightPage";

const nav = [
  { to: "/", label: "Dashboard" },
  { to: "/planner", label: "Planner" },
  { to: "/shopping", label: "Shopping" },
  { to: "/compare", label: "Compare" },
  { to: "/recipes", label: "Recipes" },
  { to: "/recipe-families", label: "Recipe families" },
  { to: "/prices", label: "Prices" },
  { to: "/stores", label: "Stores" },
  { to: "/barcode", label: "Barcode" },
  { to: "/inspiration", label: "Inspiration" },
  { to: "/grocy", label: "Grocy" },
  { to: "/aliases", label: "Nicknames" },
  { to: "/preferences", label: "Preferences" },
  { to: "/pantry", label: "Pantry" },
  { to: "/orders", label: "Orders" },
  { to: "/tonight", label: "Tonight" },
  { to: "/sync", label: "Sync" },
];

export default function App() {
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <h1>Spisordning</h1>
          <p className="tagline">Household food brain</p>
        </div>
        <nav className="nav">
          {nav.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) => (isActive ? "nav-link active" : "nav-link")}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <main className="content">
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/planner" element={<PlannerPage />} />
          <Route path="/shopping" element={<ShoppingPage />} />
          <Route path="/compare" element={<ComparePage />} />
          <Route path="/recipes" element={<RecipesPage />} />
          <Route path="/recipe-families" element={<RecipeFamilyPage />} />
          <Route path="/prices" element={<PricesPage />} />
          <Route path="/stores" element={<StoreLocatorPage />} />
          <Route path="/barcode" element={<BarcodePage />} />
          <Route path="/inspiration" element={<InspirationPage />} />
          <Route path="/grocy" element={<GrocyPage />} />
          <Route path="/aliases" element={<AliasesPage />} />
          <Route path="/preferences" element={<PreferencesPage />} />
          <Route path="/pantry" element={<PantryPage />} />
          <Route path="/orders" element={<OrdersPage />} />
          <Route path="/tonight" element={<TonightPage />} />
          <Route path="/sync" element={<SyncPage />} />
          <Route path="*" element={<PlannerPage />} />
        </Routes>
      </main>
    </div>
  );
}

import { BrowserRouter, Route, Routes, Navigate } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import IncidentList from './pages/IncidentList'
import IncidentDetail from './pages/IncidentDetail'
import NewIncident from './pages/NewIncident'
import StatusPage from './pages/StatusPage'
import AlertList from './pages/AlertList'
import Settings from './pages/Settings'
import Login from './pages/Login'

export default function App() {
    return (
        <BrowserRouter>
            <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/incidents" element={<IncidentList />} />
                <Route path="/incidents/new" element={<NewIncident />} />
                <Route path="/incidents/:id" element={<IncidentDetail />} />
                <Route path="/status" element={<StatusPage />} />
                <Route path="/alerts" element={<AlertList />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="/login" element={<Login />} />
                <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
        </BrowserRouter>
    )
}

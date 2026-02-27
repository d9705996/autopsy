import {
    LoginPage,
    LoginForm,
} from '@patternfly/react-core'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

export default function Login() {
    const navigate = useNavigate()
    const [email, setEmail] = useState('')
    const [password, setPassword] = useState('')
    const [isLoading, setIsLoading] = useState(false)
    const [errorMessage, setErrorMessage] = useState('')

    const handleSubmit = async (event: React.MouseEvent<HTMLButtonElement>) => {
        event.preventDefault()
        setIsLoading(true)
        setErrorMessage('')
        try {
            const res = await fetch('/api/v1/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email, password }),
            })
            if (!res.ok) {
                setErrorMessage('Invalid credentials')
                return
            }
            navigate('/')
        } catch {
            setErrorMessage('Network error — please try again')
        } finally {
            setIsLoading(false)
        }
    }

    return (
        <LoginPage
            footerListItems={null}
            textContent="Autopsy — Incident Response Platform"
            loginTitle="Log in to your account"
        >
            <LoginForm
                showHelperText={!!errorMessage}
                helperText={errorMessage}
                usernameLabel="Email"
                usernameValue={email}
                onChangeUsername={(_event, value) => setEmail(value)}
                passwordLabel="Password"
                passwordValue={password}
                onChangePassword={(_event, value) => setPassword(value)}
                onLoginButtonClick={handleSubmit}
                isLoginButtonDisabled={isLoading}
            />
        </LoginPage>
    )
}

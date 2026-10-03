import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { AuthLayout } from "@/layouts/AuthLayout"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"

export default function CompanyAliasLogin() {
  const [alias, setAlias] = useState("")
  const navigate = useNavigate()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (alias.trim()) {
      navigate(`/login/${encodeURIComponent(alias.trim())}`)
    }
  }

  return (
    <AuthLayout>
      <Card>
        <CardHeader className="text-center">
          <CardTitle className="text-xl">Enter your company alias</CardTitle>
          <CardDescription>
            Enter your company alias to continue
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="alias">Company Alias</FieldLabel>
                <Input
                  id="alias"
                  type="text"
                  placeholder="acme"
                  required
                  autoFocus
                  value={alias}
                  onChange={(e) => setAlias(e.target.value)}
                />
              </Field>
              <Field>
                <Button type="submit">Continue</Button>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}




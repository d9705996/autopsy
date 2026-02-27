import { Page, PageSection, Content, Title } from '@patternfly/react-core'
import { useParams } from 'react-router-dom'

export default function IncidentDetail() {
    const { id } = useParams<{ id: string }>()
    return (
        <Page>
            <PageSection>
                
                    <Title headingLevel="h1">Incident {id}</Title>
                    <Content>Incident detail view.</Content>
                
            </PageSection>
        </Page>
    )
}

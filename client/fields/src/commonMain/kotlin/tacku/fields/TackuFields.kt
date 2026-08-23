package tacku.fields

import io.github.youndie.kompot.KompotComponent
import io.github.youndie.kompot.form.FormFieldDefinition
import kotlinx.serialization.modules.SerializersModule
import kotlinx.serialization.modules.polymorphic
import kotlinx.serialization.modules.subclass

/**
 * How this deployment's own types join the vocabulary at runtime.
 *
 * Every name appears in three places and each is a different half of the same statement: this module
 * registers it with the serializer, the profile declares it for anything that validates, and the
 * renderer registry maps it to something that draws. A name present in one and missing from another
 * is exactly the failure this extension exists to make impossible to write by accident, so the set
 * is checked by a test rather than kept in step by hand.
 *
 * The two entries cost different things when they are missing, and the difference is the point of
 * keeping them side by side. Without `date_field` a form is not parsed at all (§2.2). Without
 * the multiline pair leaves the definition as `text_field` and touches nothing but the hierarchy
 * that survives.
 */
val tackuFieldsSerializersModule: SerializersModule =
    SerializersModule {
        polymorphic(KompotComponent::class) {
            subclass(DateInput::class, DateInput.serializer())
        }
        polymorphic(FormFieldDefinition::class) {
            subclass(DateField::class, DateField.serializer())
        }
    }
